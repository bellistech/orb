package ui

import (
	"fmt"
	"net/netip"
	"sort"
	"strings"

	"orb/internal/canvas"
	"orb/internal/sys"
)

func netGrid(w int) []row {
	side := slot{nil, 0, []slot{{dash(drawNet), 55, nil}, {drawSockStats, 45, nil}}}
	switch {
	case w >= 180:
		side.weight = 28
		return []row{{1, []slot{side, {drawListen, 32, nil}, {drawConns, 40, nil}}}}
	case w >= 100:
		side.weight = 45
		return []row{
			{45, []slot{{drawListen, 55, nil}, side}},
			{55, []slot{{drawConns, 1, nil}}},
		}
	default:
		return []row{
			{30, []slot{{drawListen, 1, nil}}},
			{30, []slot{{drawConns, 1, nil}}},
			{20, []slot{{dash(drawNet), 1, nil}}},
			{20, []slot{{drawSockStats, 1, nil}}},
		}
	}
}

func protoColor(t *Theme, proto string) canvas.Color {
	if strings.HasPrefix(proto, "udp") {
		return t.Disk
	}
	return t.Net
}

func stateColor(t *Theme, state string) canvas.Color {
	switch state {
	case "ESTAB":
		return t.Net
	case "LISTEN", "UNCONN":
		return t.CPU
	case "SYN-SENT", "SYN-RECV":
		return t.GPU
	case "TIME-WAIT", "CLOSE":
		return t.Faint
	default: // CLOSE-WAIT, FIN-WAIT-*, LAST-ACK, CLOSING: worth a look
		return t.Heat(0.75)
	}
}

// addrText shows an address the way ss does: * for any, plain otherwise.
func addrText(a netip.Addr) string {
	if !a.IsValid() || a.IsUnspecified() {
		return "*"
	}
	return a.String()
}

func procText(s sys.Socket) string {
	if s.PID == 0 {
		return "-"
	}
	return fmt.Sprintf("%s[%d]", s.Proc, s.PID)
}

func socketsOrWait(in canvas.Region, s *sys.Snapshot, t *Theme) bool {
	if s.Sockets == nil {
		in.Text(0, 0, "reading sockets…", t.Dim)
		return false
	}
	return true
}

func drawListen(r canvas.Region, v *View, t *Theme) {
	var ls []sys.Socket
	hidden := false
	for _, s := range v.Snap.Sockets {
		if s.Listening() {
			ls = append(ls, s)
			hidden = hidden || s.PID == 0
		}
	}
	sub := fmt.Sprintf("%d sockets", len(ls))
	if hidden && !v.Snap.AllOwners {
		sub += " · some owners hidden (not root)"
	}
	in := Frame(r, t, "Listening", sub, t.CPU)
	if in.Empty() || !socketsOrWait(in, v.Snap, t) {
		return
	}
	protoW, portW, svcW := 5, 6, 10
	procW := min(24, max(in.W/4, 8))
	addrW := in.W - protoW - portW - svcW - procW - 4
	cols := []struct {
		x    int
		name string
	}{{0, "PROTO"}, {protoW + 1, "PORT"}, {protoW + portW + 2, "SERVICE"}, {protoW + portW + svcW + 3, "ADDRESS"}, {in.W - procW, "PROCESS"}}
	for _, c := range cols {
		in.Text(c.x, 0, c.name, t.Faint)
	}
	rows := in.H - 1
	for i, s := range ls {
		y := i + 1
		if i == rows-1 && len(ls) > rows {
			in.Text(0, y, fmt.Sprintf("+%d more", len(ls)-i), t.Faint)
			break
		}
		if y >= in.H {
			break
		}
		port := s.Local.Port()
		in.Text(0, y, s.Proto, protoColor(t, s.Proto))
		in.Bold(protoW+1, y, fmt.Sprintf("%*d", portW, port), protoColor(t, s.Proto))
		in.Text(protoW+portW+2, y, trunc(sys.Service(s.Proto, port), svcW), t.Dim)
		a := s.Local.Addr()
		col := t.FG
		if a.IsLoopback() {
			col = t.Dim
		}
		in.Text(protoW+portW+svcW+3, y, trunc(addrText(a), addrW), col)
		pc := t.FG
		if s.PID == 0 {
			pc = t.Faint
		}
		in.Text(in.W-procW, y, trunc(procText(s), procW), pc)
	}
}

func drawConns(r canvas.Region, v *View, t *Theme) {
	var cs []sys.Socket
	est := 0
	for _, s := range v.Snap.Sockets {
		if !s.Listening() {
			cs = append(cs, s)
			if s.State == "ESTAB" {
				est++
			}
		}
	}
	// Established first, then by process and peer.
	sort.SliceStable(cs, func(i, j int) bool {
		return (cs[i].State == "ESTAB") && cs[j].State != "ESTAB"
	})
	in := Frame(r, t, "Connections", fmt.Sprintf("%d · %d established", len(cs), est), t.Net)
	if in.Empty() || !socketsOrWait(in, v.Snap, t) {
		return
	}
	protoW, stateW := 5, 11
	procW := min(22, max(in.W/5, 8))
	rest := in.W - protoW - stateW - procW - 3
	showLocal := rest >= 50
	localW, remoteW := 0, rest
	if showLocal {
		localW = rest * 45 / 100
		remoteW = rest - localW - 3
	}
	x := 0
	in.Text(x, 0, "PROTO", t.Faint)
	x += protoW + 1
	in.Text(x, 0, "STATE", t.Faint)
	x += stateW + 1
	if showLocal {
		in.Text(x, 0, "LOCAL", t.Faint)
		x += localW + 3
	}
	in.Text(x, 0, "PEER", t.Faint)
	in.Text(in.W-procW, 0, "PROCESS", t.Faint)
	if len(cs) == 0 {
		in.Text(0, 1, "no connections", t.Dim)
		return
	}
	rows := in.H - 1
	for i, s := range cs {
		y := i + 1
		if i == rows-1 && len(cs) > rows {
			in.Text(0, y, fmt.Sprintf("+%d more", len(cs)-i), t.Faint)
			break
		}
		if y >= in.H {
			break
		}
		x := 0
		in.Text(x, y, s.Proto, protoColor(t, s.Proto))
		x += protoW + 1
		in.Text(x, y, s.State, stateColor(t, s.State))
		x += stateW + 1
		if showLocal {
			in.Text(x, y, trunc(s.Local.String(), localW), t.Dim)
			in.Text(x+localW+1, y, "→", t.Faint)
			x += localW + 3
		}
		peer := s.Remote.String()
		svc := sys.Service(s.Proto, s.Remote.Port())
		if svc == "" {
			svc = sys.Service(s.Proto, s.Local.Port())
		}
		px := in.Text(x, y, trunc(peer, remoteW), t.FG)
		if svc != "" && px+1+len(svc) < x+remoteW {
			in.Text(px+1, y, svc, t.Dim)
		}
		pc := t.FG
		if s.PID == 0 {
			pc = t.Faint
		}
		in.Text(in.W-procW, y, trunc(procText(s), procW), pc)
	}
}

var stateOrder = []string{"ESTAB", "LISTEN", "UNCONN", "SYN-SENT", "SYN-RECV", "FIN-WAIT-1", "FIN-WAIT-2", "CLOSE-WAIT", "LAST-ACK", "CLOSING", "TIME-WAIT", "CLOSE"}

func drawSockStats(r canvas.Region, v *View, t *Theme) {
	counts := map[string]int{}
	peers := map[netip.Addr]int{}
	for _, s := range v.Snap.Sockets {
		counts[s.State]++
		if s.State == "ESTAB" && !s.Remote.Addr().IsLoopback() {
			peers[s.Remote.Addr()]++
		}
	}
	in := Frame(r, t, "Sockets", fmt.Sprintf("%d total", len(v.Snap.Sockets)), t.GPU)
	if in.Empty() || !socketsOrWait(in, v.Snap, t) {
		return
	}
	most := 1
	for _, n := range counts {
		most = max(most, n)
	}
	labelW, numW := 11, 5
	y := 0
	for _, st := range stateOrder {
		n := counts[st]
		if n == 0 || y >= in.H {
			continue
		}
		in.Text(0, y, st, stateColor(t, st))
		gw := in.W - labelW - numW - 1
		Gauge(in, labelW, y, gw, float64(n)/float64(most), t, func(float64) canvas.Color { return stateColor(t, st) })
		in.TextRight(in.W, y, fmt.Sprint(n), t.FG)
		y++
	}
	if len(peers) == 0 || y+2 >= in.H {
		return
	}
	type peer struct {
		a netip.Addr
		n int
	}
	var ps []peer
	for a, n := range peers {
		ps = append(ps, peer{a, n})
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].n > ps[j].n || ps[i].n == ps[j].n && ps[i].a.Less(ps[j].a) })
	y++
	in.Text(0, y, "top peers", t.Faint)
	y++
	for _, p := range ps {
		if y >= in.H {
			break
		}
		in.Text(0, y, trunc(p.a.String(), in.W-numW-1), t.FG)
		in.TextRight(in.W, y, fmt.Sprint(p.n), t.Net)
		y++
	}
}
