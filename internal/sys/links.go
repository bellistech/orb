package sys

import (
	"net/netip"
	"os"
	"sort"
	"strings"
)

// Links is the layer 2 and 3 view of the host: interfaces read like switch
// ports, the bridges' learned MAC tables, the ARP/NDP neighbor table, the
// main routing table and LLDP neighbors.
type Links struct {
	Links  []Link
	Neigh  []Neighbor
	Routes []Route
	FDB    []FDBEntry
	LLDP   []LLDPNeighbor
	// LLDPOn is true when LLDP frames are being received (orb started as
	// root); otherwise the LLDP pane says why it is empty.
	LLDPOn bool
}

// Link is one interface.
type Link struct {
	Name      string
	Index     int
	Kind      string // ether, wifi, bridge, bond, vlan, veth, wireguard, tun, loopback, other
	Up        bool
	Carrier   bool
	SpeedMbps int // 0 when the driver does not say
	Duplex    string
	MTU       int
	MAC       string
	Master    string   // the bridge or bond it is a port of
	Ports     []string // a bridge's or bond's member ports

	RxRate, TxRate    float64   // bytes/s
	ErrRate, DropRate float64   // per second, rx and tx together
	Errors, Drops     uint64    // since boot, rx and tx together
	Hist              []float64 // rx+tx bytes/s, since the links page opened
	Addrs             []string
}

// Neighbor is one ARP (IPv4) or NDP (IPv6) entry.
type Neighbor struct {
	Addr  netip.Addr
	MAC   string
	Dev   string
	State string // REACHABLE, STALE, DELAY, PROBE, FAILED, INCOMPLETE, PERMANENT, NOARP
}

// Route is one main-table unicast route.
type Route struct {
	Dst    netip.Prefix
	Via    netip.Addr // invalid when directly connected
	Dev    string
	Metric int
	Proto  string // kernel, boot, static, dhcp, ra, bird, bgp, ...
}

// FDBEntry is a MAC a bridge learned on one of its ports.
type FDBEntry struct {
	MAC    string
	Port   string
	Bridge string
	VLAN   int
}

// LLDPNeighbor is what a directly attached device announces over LLDP.
type LLDPNeighbor struct {
	Iface    string // our interface it was heard on
	Chassis  string
	Port     string
	PortDesc string
	Name     string // system name
	Desc     string // system description
	Caps     []string
	Mgmt     []string
	TTL      int // seconds the announcement is valid for
	Age      int // seconds since it was heard
}

type linkCounters struct{ rx, tx, errs, drops uint64 }

// readLinks reads every interface from /sys/class/net.
func readLinks() ([]Link, map[string]linkCounters) {
	ents, _ := os.ReadDir("/sys/class/net")
	var out []Link
	counters := map[string]linkCounters{}
	for _, e := range ents {
		name := e.Name()
		if !validIfName(name) {
			continue
		}
		d := "/sys/class/net/" + name + "/"
		idx, _ := readInt(d + "ifindex")
		mtu, _ := readInt(d + "mtu")
		l := Link{
			Name:    name,
			Index:   int(idx),
			Kind:    linkKind(d, name),
			Carrier: readString(d+"carrier") == "1",
			MTU:     int(mtu),
			MAC:     readString(d + "address"),
			Duplex:  readString(d + "duplex"),
		}
		oper := readString(d + "operstate")
		// WireGuard and tun report "unknown": up means carrying traffic.
		l.Up = oper == "up" || (oper == "unknown" && l.Carrier)
		if sp, ok := readInt(d + "speed"); ok && sp > 0 && sp < 10_000_000 {
			l.SpeedMbps = int(sp)
		}
		if t, err := os.Readlink(d + "master"); err == nil {
			l.Master = baseName(t)
		}
		if l.Kind == "bridge" {
			l.Ports = dirNames(d + "brif")
		}
		if l.Kind == "bond" {
			l.Ports = strings.Fields(readString(d + "bonding/slaves"))
		}
		st := d + "statistics/"
		rxb, _ := readUint(st + "rx_bytes")
		txb, _ := readUint(st + "tx_bytes")
		rxe, _ := readUint(st + "rx_errors")
		txe, _ := readUint(st + "tx_errors")
		rxd, _ := readUint(st + "rx_dropped")
		txd, _ := readUint(st + "tx_dropped")
		counters[name] = linkCounters{rxb, txb, rxe + txe, rxd + txd}
		l.Errors, l.Drops = rxe+txe, rxd+txd
		if l.Kind != "veth" && l.Kind != "loopback" {
			l.Addrs = ifAddrs(name)
		}
		out = append(out, l)
	}
	sortLinks(out)
	return out, counters
}

// validIfName keeps names the kernel could have made: 1-15 bytes, no slash.
func validIfName(s string) bool {
	return s != "" && len(s) <= 15 && !strings.ContainsAny(s, "/\x00") && s != "." && s != ".."
}

func baseName(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

func dirNames(path string) []string {
	ents, _ := os.ReadDir(path)
	var out []string
	for _, e := range ents {
		out = append(out, e.Name())
	}
	return out
}

func linkKind(d, name string) string {
	exists := func(p string) bool { _, err := os.Stat(d + p); return err == nil }
	uevent := readString(d + "uevent")
	switch {
	case name == "lo" || readString(d+"type") == "772":
		return "loopback"
	case exists("bridge"):
		return "bridge"
	case exists("bonding"):
		return "bond"
	case exists("wireless") || exists("phy80211"):
		return "wifi"
	case strings.Contains(uevent, "DEVTYPE=wireguard"):
		return "wireguard"
	case strings.Contains(uevent, "DEVTYPE=vlan"):
		return "vlan"
	case exists("tun_flags"):
		return "tun"
	case strings.HasPrefix(name, "veth") || strings.Contains(uevent, "DEVTYPE=veth"):
		return "veth"
	case exists("device"):
		return "ether"
	}
	return "other"
}

var kindOrder = map[string]int{"ether": 0, "wifi": 1, "bond": 2, "vlan": 3, "bridge": 4, "wireguard": 5, "tun": 6, "other": 7, "veth": 8, "loopback": 9}

// sortLinks puts physical ports first and container plumbing last.
func sortLinks(ls []Link) {
	sort.SliceStable(ls, func(i, j int) bool {
		a, b := kindOrder[ls[i].Kind], kindOrder[ls[j].Kind]
		if a != b {
			return a < b
		}
		return ls[i].Name < ls[j].Name
	})
}

// linkRates fills rates from the previous counters over dt seconds.
func linkRates(ls []Link, prev, cur map[string]linkCounters, dt float64) {
	for i := range ls {
		p, ok := prev[ls[i].Name]
		c := cur[ls[i].Name]
		if !ok || dt <= 0 {
			continue
		}
		ls[i].RxRate = rate(p.rx, c.rx, dt)
		ls[i].TxRate = rate(p.tx, c.tx, dt)
		ls[i].ErrRate = rate(p.errs, c.errs, dt)
		ls[i].DropRate = rate(p.drops, c.drops, dt)
	}
}
