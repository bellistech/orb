package sys

import (
	"net"
	"os"
	"strings"
)

// NetIf is one interface's current throughput in bytes/second.
type NetIf struct {
	Name           string
	Up             bool
	RxRate, TxRate float64
	Addrs          []string // IPv4 first, then global IPv6; no link-local
}

type netCounters struct{ rx, tx uint64 }

var virtualPrefixes = []string{"veth", "docker", "br-", "virbr", "cni", "lo"}

// readNet returns byte counters for interfaces worth showing: physical
// devices always, virtual ones unless they are container plumbing.
func readNet() map[string]netCounters {
	out := map[string]netCounters{}
	lines := strings.Split(readString("/proc/net/dev"), "\n")
	for _, line := range lines {
		name, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		if skipIf(name) {
			continue
		}
		f := strings.Fields(rest)
		if len(f) < 9 {
			continue
		}
		out[name] = netCounters{rx: atou(f[0]), tx: atou(f[8])}
	}
	return out
}

func skipIf(name string) bool {
	if _, err := os.Stat("/sys/class/net/" + name + "/device"); err == nil {
		return false
	}
	for _, p := range virtualPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func ifUp(name string) bool {
	return readString("/sys/class/net/"+name+"/operstate") == "up"
}

// ifAddrs returns the interface's displayable addresses.
func ifAddrs(name string) []string {
	ifc, err := net.InterfaceByName(name)
	if err != nil {
		return nil
	}
	addrs, err := ifc.Addrs()
	if err != nil {
		return nil
	}
	return pickAddrs(addrs)
}

// pickAddrs orders IPv4 before IPv6 and drops link-local and unparsable
// entries, so the first address is the one worth showing.
func pickAddrs(addrs []net.Addr) []string {
	var v4, v6 []string
	for _, a := range addrs {
		ipn, ok := a.(*net.IPNet)
		if !ok || ipn.IP == nil || ipn.IP.IsLinkLocalUnicast() || ipn.IP.IsLoopback() {
			continue
		}
		if ip4 := ipn.IP.To4(); ip4 != nil {
			v4 = append(v4, ip4.String())
		} else {
			v6 = append(v6, ipn.IP.String())
		}
	}
	return append(v4, v6...)
}
