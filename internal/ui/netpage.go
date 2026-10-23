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
		side.weight = 38
		// Connections are few and long; listening sockets are many and
		// short, so they take the full width and flow into columns.
		return []row{
			{45, []slot{{drawConns, 62, nil}, side}},
			{55, []slot{{drawListen, 1, nil}}},
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
	// Past two minimum-width columns the list flows into columns.
	ncol := max(1, min((in.W+listenGap)/(listenColW+listenGap), (len(ls)+in.H-2)/max(in.H-1, 1)))
	colW := (in.W - (ncol-1)*listenGap) / ncol
	per := in.H - 1
	for c := 0; c < ncol; c++ {
		x := c * (colW + listenGap)
		if c == ncol-1 {
			colW = in.W - x
		}
		lo, hi := min(c*per, len(ls)), min((c+1)*per, len(ls))
		if c == ncol-1 {
			hi = len(ls) // the last column counts what is left over
		}
		drawListenCol(in.Sub(x, 0, colW, in.H), ls[lo:hi], t)
	}
}

// listenColW is the narrowest listening column worth splitting into.
const (
	listenColW = 64
	listenGap  = 3
)

// drawListenCol draws a header and as many of ls as fit; when more than
// fit, the last row says how many are left.
func drawListenCol(in canvas.Region, ls []sys.Socket, t *Theme) {
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

// listenKey identifies a listening port across IPv4 and IPv6: "tcp:22".
func listenKey(proto string, port uint16) string {
	return strings.TrimSuffix(proto, "6") + ":" + fmt.Sprint(port)
}

// serverIsLocal reports whether a connection's server end is this host: its
// local port is one we listen on (inbound). Otherwise the peer serves; UDP
// with no listener on either side takes the lower port as the server.
func serverIsLocal(s sys.Socket, listening map[string]bool) bool {
	if listening[listenKey(s.Proto, s.Local.Port())] {
		return true
	}
	if strings.HasPrefix(s.Proto, "udp") {
		return s.Local.Port() < s.Remote.Port()
	}
	return false
}

func drawConns(r canvas.Region, v *View, t *Theme) {
	var cs []sys.Socket
	est := 0
	listening := map[string]bool{}
	for _, s := range v.Snap.Sockets {
		if s.Listening() {
			listening[listenKey(s.Proto, s.Local.Port())] = true
		}
	}
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
		localW = min(rest*45/100, 46) // the longest IPv6 [addr]:port
		remoteW = rest - localW - 3
	}
	x := 0
	in.Text(x, 0, "PROTO", t.Faint)
	x += protoW + 1
	in.Text(x, 0, "STATE", t.Faint)
	x += stateW + 1
	// Arrows run client (ephemeral port) -> server (its service port).
	if showLocal {
		in.Text(x, 0, "CLIENT", t.Faint)
		x += localW + 3
		in.Text(x, 0, "SERVER", t.Faint)
	} else {
		in.Text(x, 0, "PEER", t.Faint)
	}
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
		client, server := s.Local, s.Remote
		if serverIsLocal(s, listening) {
			client, server = s.Remote, s.Local
		}
		svc := sys.Service(s.Proto, server.Port())
		var px int
		if showLocal {
			in.Text(x, y, trunc(client.String(), localW), t.Dim)
			in.Text(x+localW+1, y, "→", t.Faint)
			x += localW + 3
			px = in.Text(x, y, trunc(server.String(), remoteW), t.FG)
		} else {
			// Only the peer fits: the arrow says which way it points.
			arrow := "→ " // we are the client, the peer serves
			if server == s.Local {
				arrow = "← " // the peer is a client of ours
			}
			px = in.Text(x, y, arrow, t.Faint)
			px = in.Text(px, y, trunc(s.Remote.String(), remoteW-2), t.FG)
		}
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
