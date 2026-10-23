package sys

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"sort"
)

// Netlink message bodies, parsed by hand: syscall.ParseNetlinkRouteAttr
// does not know neighbor messages. Every length the kernel reports is
// checked before it is used.

const (
	afInet   = 2
	afInet6  = 10
	afBridge = 7

	ndmsgLen = 12 // family, pad, pad, ifindex, state, flags, type
	rtmsgLen = 12 // family, dst_len, src_len, tos, table, protocol, scope, type, flags

	ndaDst    = 1
	ndaLLAddr = 2
	ndaVLAN   = 5
	ndaMaster = 9

	rtaDst      = 1
	rtaOIF      = 4
	rtaGateway  = 5
	rtaPriority = 6
	rtaTable    = 15

	rtTableMain = 254
	rtnUnicast  = 1

	nudIncomplete = 0x01
	nudReachable  = 0x02
	nudStale      = 0x04
	nudDelay      = 0x08
	nudProbe      = 0x10
	nudFailed     = 0x20
	nudNoARP      = 0x40
	nudPermanent  = 0x80

	ntfSelf = 0x02
)

// rtAttrs splits a run of route attributes (len, type, value, 4-aligned).
// A malformed length ends the run; it never reads past b.
func rtAttrs(b []byte) map[uint16][]byte {
	out := map[uint16][]byte{}
	for len(b) >= 4 {
		l := int(binary.NativeEndian.Uint16(b[0:2]))
		typ := binary.NativeEndian.Uint16(b[2:4]) &^ 0xc000 // drop NLA_F_NESTED/NET_BYTEORDER
		if l < 4 || l > len(b) {
			break
		}
		if _, dup := out[typ]; !dup {
			out[typ] = b[4:l]
		}
		al := (l + 3) &^ 3
		if al > len(b) {
			break
		}
		b = b[al:]
	}
	return out
}

func ipOf(b []byte) (netip.Addr, bool) {
	a, ok := netip.AddrFromSlice(b)
	if !ok || !(len(b) == 4 || len(b) == 16) {
		return netip.Addr{}, false
	}
	return a.Unmap(), true
}

func macOf(b []byte) string {
	if len(b) != 6 {
		return ""
	}
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", b[0], b[1], b[2], b[3], b[4], b[5])
}

func nudName(s uint16) string {
	switch {
	case s&nudPermanent != 0:
		return "PERMANENT"
	case s&nudReachable != 0:
		return "REACHABLE"
	case s&nudStale != 0:
		return "STALE"
	case s&nudDelay != 0:
		return "DELAY"
	case s&nudProbe != 0:
		return "PROBE"
	case s&nudFailed != 0:
		return "FAILED"
	case s&nudIncomplete != 0:
		return "INCOMPLETE"
	case s&nudNoARP != 0:
		return "NOARP"
	}
	return "NONE"
}

// parseNeigh turns one RTM_NEWNEIGH body (AF_INET or AF_INET6) into a
// neighbor. Multicast and NOARP entries are not neighbors worth showing.
func parseNeigh(b []byte, ifname func(int) string) (Neighbor, bool) {
	if len(b) < ndmsgLen {
		return Neighbor{}, false
	}
	fam := b[0]
	if fam != afInet && fam != afInet6 {
		return Neighbor{}, false
	}
	idx := int(int32(binary.NativeEndian.Uint32(b[4:8])))
	state := binary.NativeEndian.Uint16(b[8:10])
	at := rtAttrs(b[ndmsgLen:])
	a, ok := ipOf(at[ndaDst])
	if !ok || a.IsMulticast() || state&nudNoARP != 0 || state == 0 {
		return Neighbor{}, false
	}
	return Neighbor{Addr: a, MAC: macOf(at[ndaLLAddr]), Dev: ifname(idx), State: nudName(state)}, true
}

// parseFDB turns one AF_BRIDGE RTM_NEWNEIGH body into a learned MAC.
// Permanent entries (the bridge's and ports' own addresses), self entries
// (the port device's own table) and multicast MACs are skipped.
func parseFDB(b []byte, ifname func(int) string) (FDBEntry, bool) {
	if len(b) < ndmsgLen || b[0] != afBridge {
		return FDBEntry{}, false
	}
	idx := int(int32(binary.NativeEndian.Uint32(b[4:8])))
	state := binary.NativeEndian.Uint16(b[8:10])
	flags := b[10]
	if state&nudPermanent != 0 || flags&ntfSelf != 0 {
		return FDBEntry{}, false
	}
	at := rtAttrs(b[ndmsgLen:])
	ll := at[ndaLLAddr]
	if len(ll) != 6 || ll[0]&1 != 0 {
		return FDBEntry{}, false
	}
	e := FDBEntry{MAC: macOf(ll), Port: ifname(idx)}
	if m := at[ndaMaster]; len(m) == 4 {
		e.Bridge = ifname(int(int32(binary.NativeEndian.Uint32(m))))
	}
	if v := at[ndaVLAN]; len(v) == 2 {
		e.VLAN = int(binary.NativeEndian.Uint16(v))
	}
	return e, true
}

var protoNames = map[byte]string{
	1: "redirect", 2: "kernel", 3: "boot", 4: "static", 9: "ra", 11: "zebra", 12: "bird",
	16: "dhcp", 42: "babel", 186: "bgp", 187: "isis", 188: "ospf", 189: "rip", 192: "eigrp",
}

// parseRoute turns one RTM_NEWROUTE body into a main-table unicast route.
func parseRoute(b []byte, ifname func(int) string) (Route, bool) {
	if len(b) < rtmsgLen {
		return Route{}, false
	}
	fam, dstLen, table, proto, typ := b[0], int(b[1]), uint32(b[4]), b[5], b[7]
	if (fam != afInet && fam != afInet6) || typ != rtnUnicast {
		return Route{}, false
	}
	at := rtAttrs(b[rtmsgLen:])
	if t := at[rtaTable]; len(t) == 4 {
		table = binary.NativeEndian.Uint32(t)
	}
	if table != rtTableMain {
		return Route{}, false
	}
	var dst netip.Addr
	if fam == afInet {
		dst = netip.IPv4Unspecified()
	} else {
		dst = netip.IPv6Unspecified()
	}
	if a, ok := ipOf(at[rtaDst]); ok {
		dst = a
	}
	if dstLen > dst.BitLen() {
		return Route{}, false
	}
	r := Route{Dst: netip.PrefixFrom(dst, dstLen), Proto: protoNames[proto]}
	if r.Proto == "" {
		r.Proto = fmt.Sprint(proto)
	}
	if g, ok := ipOf(at[rtaGateway]); ok {
		r.Via = g
	}
	if o := at[rtaOIF]; len(o) == 4 {
		r.Dev = ifname(int(int32(binary.NativeEndian.Uint32(o))))
	}
	if p := at[rtaPriority]; len(p) == 4 {
		r.Metric = int(binary.NativeEndian.Uint32(p))
	}
	return r, true
}

// sortRoutes: defaults first, then IPv4 before IPv6, longest prefix last.
func sortRoutes(rs []Route) {
	sort.SliceStable(rs, func(i, j int) bool {
		a, b := rs[i], rs[j]
		if (a.Dst.Bits() == 0) != (b.Dst.Bits() == 0) {
			return a.Dst.Bits() == 0
		}
		if a.Dst.Addr().Is4() != b.Dst.Addr().Is4() {
			return a.Dst.Addr().Is4()
		}
		if c := a.Dst.Addr().Compare(b.Dst.Addr()); c != 0 {
			return c < 0
		}
		return a.Metric < b.Metric
	})
}

// sortNeigh: by interface, then address.
func sortNeigh(ns []Neighbor) {
	sort.SliceStable(ns, func(i, j int) bool {
		if ns[i].Dev != ns[j].Dev {
			return ns[i].Dev < ns[j].Dev
		}
		return ns[i].Addr.Less(ns[j].Addr)
	})
}
