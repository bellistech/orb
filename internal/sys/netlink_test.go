package sys

import (
	"encoding/binary"
	"net/netip"
	"testing"
)

func attr(typ uint16, v []byte) []byte {
	l := 4 + len(v)
	b := make([]byte, (l+3)&^3)
	binary.NativeEndian.PutUint16(b[0:2], uint16(l))
	binary.NativeEndian.PutUint16(b[2:4], typ)
	copy(b[4:], v)
	return b
}

func u32(v uint32) []byte { b := make([]byte, 4); binary.NativeEndian.PutUint32(b, v); return b }

func ndmsg(fam byte, idx int32, state uint16, flags byte, attrs ...[]byte) []byte {
	b := make([]byte, ndmsgLen)
	b[0] = fam
	binary.NativeEndian.PutUint32(b[4:8], uint32(idx))
	binary.NativeEndian.PutUint16(b[8:10], state)
	b[10] = flags
	for _, a := range attrs {
		b = append(b, a...)
	}
	return b
}

func rtmsg(fam, dstLen, table, proto, typ byte, attrs ...[]byte) []byte {
	b := []byte{fam, dstLen, 0, 0, table, proto, 0, typ, 0, 0, 0, 0}
	for _, a := range attrs {
		b = append(b, a...)
	}
	return b
}

var names = func(i int) string { return map[int]string{2: "enp16s0", 5: "br0", 6: "veth1", 7: "wg0"}[i] }

func TestParseNeigh(t *testing.T) {
	mac := []byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x03}
	n, ok := parseNeigh(ndmsg(afInet, 2, nudReachable, 0, attr(ndaDst, []byte{192, 168, 210, 1}), attr(ndaLLAddr, mac)), names)
	if !ok || n.Addr != netip.MustParseAddr("192.168.210.1") || n.MAC != "02:00:00:00:00:03" || n.Dev != "enp16s0" || n.State != "REACHABLE" {
		t.Errorf("v4 neighbor = %+v, %v", n, ok)
	}
	v6 := netip.MustParseAddr("fe80::1").As16()
	if n, ok := parseNeigh(ndmsg(afInet6, 2, nudStale, 0, attr(ndaDst, v6[:])), names); !ok || n.State != "STALE" || n.MAC != "" {
		t.Errorf("v6 neighbor without lladdr = %+v, %v", n, ok)
	}
	for name, b := range map[string][]byte{
		"multicast":  ndmsg(afInet, 2, nudReachable, 0, attr(ndaDst, []byte{224, 0, 0, 1})),
		"noarp":      ndmsg(afInet, 2, nudNoARP, 0, attr(ndaDst, []byte{10, 0, 0, 1})),
		"no state":   ndmsg(afInet, 2, 0, 0, attr(ndaDst, []byte{10, 0, 0, 1})),
		"no address": ndmsg(afInet, 2, nudReachable, 0),
		"bad family": ndmsg(afBridge, 2, nudReachable, 0, attr(ndaDst, []byte{10, 0, 0, 1})),
		"short":      {afInet, 0, 0},
	} {
		if _, ok := parseNeigh(b, names); ok {
			t.Errorf("%s: parsed as a neighbor", name)
		}
	}
}

func TestParseFDB(t *testing.T) {
	mac := []byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x08}
	e, ok := parseFDB(ndmsg(afBridge, 6, nudReachable, 0x04, attr(ndaLLAddr, mac), attr(ndaMaster, u32(5)), attr(ndaVLAN, []byte{1, 0})), names)
	if !ok || e.MAC != "02:00:00:00:00:08" || e.Port != "veth1" || e.Bridge != "br0" || e.VLAN != 1 {
		t.Errorf("learned MAC = %+v, %v", e, ok)
	}
	for name, b := range map[string][]byte{
		"permanent (own address)": ndmsg(afBridge, 6, nudPermanent, 0x04, attr(ndaLLAddr, mac)),
		"self (port's own table)": ndmsg(afBridge, 6, nudReachable, ntfSelf, attr(ndaLLAddr, mac)),
		"multicast MAC":           ndmsg(afBridge, 6, nudReachable, 0x04, attr(ndaLLAddr, []byte{0x33, 0x33, 0, 0, 0, 1})),
		"short MAC":               ndmsg(afBridge, 6, nudReachable, 0x04, attr(ndaLLAddr, mac[:4])),
		"inet family":             ndmsg(afInet, 6, nudReachable, 0x04, attr(ndaLLAddr, mac)),
	} {
		if _, ok := parseFDB(b, names); ok {
			t.Errorf("%s: parsed as a learned MAC", name)
		}
	}
}

func TestParseRoute(t *testing.T) {
	r, ok := parseRoute(rtmsg(afInet, 0, rtTableMain, 4, rtnUnicast, attr(rtaGateway, []byte{192, 168, 50, 129}), attr(rtaOIF, u32(2)), attr(rtaPriority, u32(600))), names)
	if !ok || r.Dst.String() != "0.0.0.0/0" || r.Via.String() != "192.168.50.129" || r.Dev != "enp16s0" || r.Metric != 600 || r.Proto != "static" {
		t.Errorf("default route = %+v, %v", r, ok)
	}
	dst := netip.MustParseAddr("fd12:3456:789a:2::").As16()
	r, ok = parseRoute(rtmsg(afInet6, 64, 0, 186, rtnUnicast, attr(rtaDst, dst[:]), attr(rtaTable, u32(rtTableMain)), attr(rtaOIF, u32(7))), names)
	if !ok || r.Dst.String() != "fd12:3456:789a:2::/64" || r.Proto != "bgp" || r.Dev != "wg0" || r.Via.IsValid() {
		t.Errorf("bgp route (table in an attribute) = %+v, %v", r, ok)
	}
	for name, b := range map[string][]byte{
		"local table":     rtmsg(afInet, 32, 255, 2, 2, attr(rtaDst, []byte{127, 0, 0, 1})),
		"not unicast":     rtmsg(afInet, 0, rtTableMain, 2, 3),
		"prefix too long": rtmsg(afInet, 33, rtTableMain, 2, rtnUnicast, attr(rtaDst, []byte{10, 0, 0, 0})),
		"short":           {afInet, 0},
	} {
		if _, ok := parseRoute(b, names); ok {
			t.Errorf("%s: parsed as a route", name)
		}
	}
}

func TestRTAttrsStopsAtBadLengths(t *testing.T) {
	good := attr(ndaDst, []byte{10, 0, 0, 1})
	for name, b := range map[string][]byte{
		"length past end":     append(append([]byte{}, good...), 0xff, 0x00, 0x02, 0x00),
		"length below header": append(append([]byte{}, good...), 0x02, 0x00, 0x02, 0x00),
		"truncated header":    append(append([]byte{}, good...), 0x08),
	} {
		at := rtAttrs(b)
		if len(at) != 1 || len(at[ndaDst]) != 4 {
			t.Errorf("%s: attrs = %v, want only the good one", name, at)
		}
	}
}

func FuzzNetlinkParsers(f *testing.F) {
	f.Add(ndmsg(afInet, 2, nudReachable, 0, attr(ndaDst, []byte{10, 0, 0, 1})))
	f.Add(ndmsg(afBridge, 6, nudReachable, 4, attr(ndaLLAddr, []byte{0, 1, 2, 3, 4, 5}), attr(ndaMaster, u32(5))))
	f.Add(rtmsg(afInet6, 64, rtTableMain, 2, rtnUnicast, attr(rtaDst, make([]byte, 16))))
	f.Fuzz(func(t *testing.T, b []byte) {
		rtAttrs(b)
		parseNeigh(b, names)
		parseFDB(b, names)
		parseRoute(b, names)
	})
}
