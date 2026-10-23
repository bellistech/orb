package ui

import (
	"fmt"
	"sort"
	"strings"

	"orb/internal/canvas"
	"orb/internal/sys"
)

// The links page: layer 2 and 3 the way a switch or router shows it.
// Interfaces as ports, bridges with their ports and learned MACs, LLDP
// neighbors, the ARP/NDP neighbor table and the routing table.

func linksGrid(w int) []row {
	switch {
	case w >= 180:
		return []row{
			{55, []slot{{drawPorts, 48, nil}, {drawBridges, 26, nil}, {drawLLDP, 26, nil}}},
			{45, []slot{{drawNeigh, 45, nil}, {drawRoutes, 55, nil}}},
		}
	case w >= 100:
		return []row{
			{38, []slot{{drawPorts, 60, nil}, {drawLLDP, 40, nil}}},
			{32, []slot{{drawNeigh, 55, nil}, {drawBridges, 45, nil}}},
			{30, []slot{{drawRoutes, 1, nil}}},
		}
	default:
		return []row{
			{26, []slot{{drawPorts, 1, nil}}},
			{16, []slot{{drawLLDP, 1, nil}}},
			{20, []slot{{drawRoutes, 1, nil}}},
			{20, []slot{{drawNeigh, 1, nil}}},
			{18, []slot{{drawBridges, 1, nil}}},
		}
	}
}

func linksOrWait(in canvas.Region, v *View, t *Theme) *sys.Links {
	if v.Snap.Links == nil {
		in.Text(0, 0, "reading links…", t.Dim)
	}
	return v.Snap.Links
}

// speedText is a port speed the way switches print it: 1G, 10G, 100M.
func speedText(mbps int) string {
	switch {
	case mbps <= 0:
		return "-"
	case mbps >= 1000 && mbps%1000 == 0:
		return fmt.Sprintf("%dG", mbps/1000)
	case mbps >= 1000:
		return fmt.Sprintf("%.1fG", float64(mbps)/1000)
	}
	return fmt.Sprintf("%dM", mbps)
}

func kindColor(t *Theme, kind string) canvas.Color {
	switch kind {
	case "ether", "bond", "vlan":
		return t.Net
	case "wifi":
		return t.CPU
	case "bridge":
		return t.Disk
	case "wireguard", "tun":
		return t.Mem
	}
	return t.Dim
}

// util is how busy a port is: the busier direction over its line rate, or
// over its own peak when the driver reports no speed.
func util(l sys.Link) float64 {
	if l.SpeedMbps > 0 {
		return max(l.RxRate, l.TxRate) * 8 / (float64(l.SpeedMbps) * 1e6)
	}
	return 0
}

// sparkNorm scales a history for Spark: against the line rate when known,
// else against its own peak.
func sparkNorm(l sys.Link) []float64 {
	top := 0.0
	if l.SpeedMbps > 0 {
		top = float64(l.SpeedMbps) * 1e6 / 8 * 2 // rx+tx at full duplex
	}
	if top == 0 {
		for _, v := range l.Hist {
			top = max(top, v)
		}
	}
	out := make([]float64, len(l.Hist))
	if top <= 0 {
		return out
	}
	for i, v := range l.Hist {
		out[i] = v / top
	}
	return out
}

func drawPorts(r canvas.Region, v *View, t *Theme) {
	var ports []sys.Link
	up, veths := 0, 0
	if v.Snap.Links != nil {
		for _, l := range v.Snap.Links.Links {
			switch {
			case l.Kind == "veth":
				veths++ // shown under their bridge
			case l.Kind != "loopback":
				ports = append(ports, l)
				if l.Up {
					up++
				}
			}
		}
	}
	sub := fmt.Sprintf("%d up of %d", up, len(ports))
	if veths > 0 {
		sub += fmt.Sprintf(" · %d veth", veths)
	}
	in := Frame(r, t, "Interfaces", sub, t.Net)
	if in.Empty() || linksOrWait(in, v, t) == nil {
		return
	}
	nameW, kindW, spdW, mtuW, rateW := 15, 9, 5, 5, 9
	sparkW := 0
	errW := 11
	rest := in.W - (2 + nameW + kindW + spdW + mtuW + 2*rateW + errW + 7)
	if rest >= 8 {
		sparkW = min(rest, 40)
	}
	x := []int{0, 2}
	for _, w := range []int{nameW, kindW, spdW, mtuW, rateW, rateW, sparkW} {
		x = append(x, x[len(x)-1]+w+1)
	}
	for i, h := range []string{"", "PORT", "KIND", "SPEED", "MTU", "     ↓ RX", "     ↑ TX", "LOAD"} {
		in.Text(x[i], 0, h, t.Faint)
	}
	in.TextRight(in.W, 0, "ERR/DROP", t.Faint)
	for i, l := range ports {
		y := i + 1
		if y >= in.H {
			break
		}
		dot := t.Faint
		switch {
		case l.Up:
			dot = t.Net
		case l.Carrier:
			dot = t.Heat(0.6)
		}
		in.Set(x[0], y, '●', dot)
		in.Bold(x[1], y, trunc(l.Name, nameW), t.FG)
		in.Text(x[2], y, l.Kind, kindColor(t, l.Kind))
		in.TextRight(x[3]+spdW, y, speedText(l.SpeedMbps), t.FG)
		in.TextRight(x[4]+mtuW, y, fmt.Sprint(l.MTU), t.Dim)
		in.TextRight(x[5]+rateW, y, Rate(l.RxRate), t.RX)
		in.TextRight(x[6]+rateW, y, Rate(l.TxRate), t.TX)
		if sparkW > 0 {
			Spark(in, x[7], y, sparkW, sparkNorm(l), t.Heat)
		}
		ec := t.Faint
		if l.ErrRate > 0 || l.DropRate > 0 {
			ec = t.Heat(0.85) // happening now
		} else if l.Errors+l.Drops > 0 {
			ec = t.Dim
		}
		in.TextRight(in.W, y, fmt.Sprintf("%d/%d", l.Errors, l.Drops), ec)
	}
	// Under the table: the busiest port's utilization when it has a line rate.
	if len(ports)+2 < in.H {
		y := len(ports) + 2
		for _, l := range ports {
			physical := l.Kind == "ether" || l.Kind == "bond" || l.Kind == "vlan"
			if u := util(l); physical && l.SpeedMbps > 0 && l.Up {
				if y >= in.H {
					break
				}
				in.Text(2, y, trunc(l.Name, nameW), t.Dim)
				gx := 2 + nameW + 1
				gw := in.W - gx - 6
				Gauge(in, gx, y, gw, u, t, t.Heat)
				in.TextRight(in.W, y, Pct(u), t.Heat(u))
				y++
			}
		}
	}
}

func drawBridges(r canvas.Region, v *View, t *Theme) {
	lv := v.Snap.Links
	var bridges []sys.Link
	byName := map[string]sys.Link{}
	macs := map[string]int{}
	portMACs := map[string]int{}
	if lv != nil {
		for _, l := range lv.Links {
			byName[l.Name] = l
			if l.Kind == "bridge" {
				bridges = append(bridges, l)
			}
		}
		for _, e := range lv.FDB {
			macs[e.Bridge]++
			portMACs[e.Port]++
		}
	}
	learned := 0
	if lv != nil {
		learned = len(lv.FDB)
	}
	in := Frame(r, t, "Bridges", fmt.Sprintf("%d · %d MACs learned", len(bridges), learned), t.Disk)
	if in.Empty() || linksOrWait(in, v, t) == nil {
		return
	}
	if len(bridges) == 0 {
		in.Text(0, 0, "no bridges", t.Dim)
		return
	}
	// Active bridges first: they carry the ports worth seeing.
	sort.SliceStable(bridges, func(i, j int) bool { return bridges[i].Up && !bridges[j].Up })
	y := 0
	for _, b := range bridges {
		if y >= in.H {
			break
		}
		col := t.FG
		if !b.Up {
			col = t.Faint
		}
		x := in.Bold(0, y, trunc(b.Name, in.W/2), col)
		in.Text(x+1, y, plural(len(b.Ports), "port")+" · "+plural(macs[b.Name], "MAC"), t.Dim)
		y++
		for i, p := range b.Ports {
			if y >= in.H {
				break
			}
			if y == in.H-1 && i < len(b.Ports)-1 {
				in.Text(2, y, fmt.Sprintf("+%d more", len(b.Ports)-i), t.Faint)
				y++
				break
			}
			branch := "├ "
			if i == len(b.Ports)-1 {
				branch = "└ "
			}
			px := in.Text(1, y, branch, t.Faint)
			pl := byName[p]
			pc := t.Dim
			if pl.Up {
				pc = t.FG
			}
			in.Text(px, y, trunc(p, 14), pc)
			if n := portMACs[p]; n > 0 {
				in.Text(px+15, y, fmt.Sprintf("%d mac", n), t.Disk)
			}
			in.TextRight(in.W, y, Rate(pl.RxRate+pl.TxRate), t.Dim)
			y++
		}
	}
}

func neighColor(t *Theme, state string) canvas.Color {
	switch state {
	case "REACHABLE", "PERMANENT":
		return t.Net
	case "STALE":
		return t.Dim
	case "DELAY", "PROBE":
		return t.GPU
	case "FAILED", "INCOMPLETE":
		return t.Heat(0.85)
	}
	return t.Faint
}

func drawNeigh(r canvas.Region, v *View, t *Theme) {
	lv := v.Snap.Links
	var ns []sys.Neighbor
	reach := 0
	if lv != nil {
		// Global and IPv4 first; link-local IPv6 after: same hosts, less to read.
		for pass := range 2 {
			for _, n := range lv.Neigh {
				if n.Addr.IsLinkLocalUnicast() == (pass == 1) {
					ns = append(ns, n)
				}
			}
		}
		for _, n := range ns {
			if n.State == "REACHABLE" {
				reach++
			}
		}
	}
	in := Frame(r, t, "Neighbors", fmt.Sprintf("ARP/NDP · %d · %d reachable", len(ns), reach), t.GPU)
	if in.Empty() || linksOrWait(in, v, t) == nil {
		return
	}
	// Address up to an IPv6 address's 39 characters; the rest to the device.
	stateW, macW := 10, 17
	showMAC := in.W-stateW-macW-3 >= 16+8
	room := in.W - stateW - 2
	if showMAC {
		room -= macW + 1
	}
	addrW := min(39, room*2/3)
	devW := room - addrW
	in.Text(0, 0, "ADDRESS", t.Faint)
	x := addrW + 1
	if showMAC {
		in.Text(x, 0, "MAC", t.Faint)
		x += macW + 1
	}
	in.Text(x, 0, "DEV", t.Faint)
	in.TextRight(in.W, 0, "STATE", t.Faint)
	if len(ns) == 0 {
		in.Text(0, 1, "no neighbors", t.Dim)
		return
	}
	for i, n := range ns {
		y := i + 1
		if y >= in.H {
			break
		}
		if y == in.H-1 && i < len(ns)-1 {
			in.Text(0, y, fmt.Sprintf("+%d more", len(ns)-i), t.Faint)
			break
		}
		ac := t.FG
		if n.Addr.IsLinkLocalUnicast() {
			ac = t.Dim
		}
		in.Text(0, y, trunc(n.Addr.String(), addrW), ac)
		x := addrW + 1
		if showMAC {
			mac := n.MAC
			if mac == "" {
				mac = "-"
			}
			in.Text(x, y, mac, t.Dim)
			x += macW + 1
		}
		in.Text(x, y, trunc(n.Dev, devW), t.Dim)
		in.TextRight(in.W, y, n.State, neighColor(t, n.State))
	}
}

func protoRouteColor(t *Theme, proto string) canvas.Color {
	switch proto {
	case "kernel":
		return t.Faint
	case "bgp", "bird", "ospf", "isis", "zebra", "babel", "rip", "eigrp":
		return t.Mem // learned from a routing protocol
	case "static", "boot":
		return t.Disk
	case "dhcp", "ra":
		return t.CPU
	}
	return t.Dim
}

func drawRoutes(r canvas.Region, v *View, t *Theme) {
	lv := v.Snap.Links
	v4, v6 := 0, 0
	if lv != nil {
		for _, rt := range lv.Routes {
			if rt.Dst.Addr().Is4() {
				v4++
			} else {
				v6++
			}
		}
	}
	in := Frame(r, t, "Routes", fmt.Sprintf("main table · %d IPv4 · %d IPv6", v4, v6), t.Mem)
	if in.Empty() || linksOrWait(in, v, t) == nil {
		return
	}
	// Destinations and next hops up to IPv6 length; the rest to the device.
	protoW, metW := 7, 6
	room := in.W - protoW - metW - 4
	dstW := min(43, max(room*2/5, 10))
	viaW := min(39, max(room*2/5, 9))
	devW := room - dstW - viaW
	in.Text(0, 0, "DESTINATION", t.Faint)
	in.Text(dstW+1, 0, "VIA", t.Faint)
	in.Text(dstW+viaW+2, 0, "DEV", t.Faint)
	in.TextRight(dstW+viaW+devW+3+metW, 0, "METRIC", t.Faint)
	in.TextRight(in.W, 0, "PROTO", t.Faint)
	for i, rt := range lv.Routes {
		y := i + 1
		if y >= in.H {
			break
		}
		if y == in.H-1 && i < len(lv.Routes)-1 {
			in.Text(0, y, fmt.Sprintf("+%d more", len(lv.Routes)-i), t.Faint)
			break
		}
		def := rt.Dst.Bits() == 0
		dst := rt.Dst.String()
		if def {
			dst = "default"
			if rt.Dst.Addr().Is6() {
				dst = "default (v6)"
			}
			in.Bold(0, y, dst, t.Host)
		} else {
			in.Text(0, y, trunc(dst, dstW), t.FG)
		}
		via, vc := "connected", t.Faint
		if rt.Via.IsValid() {
			via, vc = rt.Via.String(), t.FG
		}
		in.Text(dstW+1, y, trunc(via, viaW), vc)
		in.Text(dstW+viaW+2, y, trunc(rt.Dev, devW), t.Dim)
		in.TextRight(dstW+viaW+devW+3+metW, y, fmt.Sprint(rt.Metric), t.Dim)
		in.TextRight(in.W, y, rt.Proto, protoRouteColor(t, rt.Proto))
	}
}

func drawLLDP(r canvas.Region, v *View, t *Theme) {
	lv := v.Snap.Links
	sub := ""
	if lv != nil {
		sub = plural(len(lv.LLDP), "neighbor")
	}
	in := Frame(r, t, "LLDP", sub, t.Host)
	if in.Empty() || linksOrWait(in, v, t) == nil {
		return
	}
	if !lv.LLDPOn {
		in.Text(0, 0, "not listening: LLDP needs root", t.Dim)
		in.Text(0, 1, "run orb with sudo to hear neighbors", t.Faint)
		return
	}
	if len(lv.LLDP) == 0 {
		in.Text(0, 0, "listening; nothing heard yet", t.Dim)
		in.Text(0, 1, "neighbors announce every 30 s", t.Faint)
		return
	}
	y := 0
	line := func(label, val string, col canvas.Color) {
		if y >= in.H || val == "" {
			return
		}
		in.Text(2, y, label, t.Faint)
		in.Text(12, y, trunc(val, in.W-12), col)
		y++
	}
	for _, n := range lv.LLDP {
		if y >= in.H {
			break
		}
		name := n.Name
		if name == "" {
			name = n.Chassis
		}
		x := in.Text(0, y, n.Iface, t.Dim)
		x = in.Text(x+1, y, "←", t.Faint)
		in.Bold(x+1, y, trunc(name, in.W-x-1), t.Host)
		age := fmt.Sprintf("%ds", n.Age)
		ac := t.Dim
		if n.TTL > 0 && n.Age > n.TTL*3/4 {
			ac = t.Heat(0.8) // about to expire
		}
		in.TextRight(in.W, y, age, ac)
		y++
		port := n.Port
		if n.PortDesc != "" && n.PortDesc != n.Port {
			port += " (" + n.PortDesc + ")"
		}
		line("port", port, t.FG)
		line("chassis", n.Chassis, t.Dim)
		line("mgmt", strings.Join(n.Mgmt, " "), t.FG)
		line("caps", strings.Join(n.Caps, " "), t.Mem)
		line("system", n.Desc, t.Dim)
		y++
	}
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
