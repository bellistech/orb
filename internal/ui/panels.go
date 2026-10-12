package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"orb/internal/canvas"
	"orb/internal/sys"
)

// heat returns the theme's heat function, for passing to widgets.
func (t *Theme) heat() func(float64) canvas.Color { return t.Heat }

// shade returns a function that brightens accent with level, for graphs
// whose height is relative (autoscaled) rather than a utilization.
func shade(t *Theme, accent canvas.Color) func(float64) canvas.Color {
	base := Mix(accent, t.Panel, 0.6)
	return func(l float64) canvas.Color { return Mix(base, accent, l) }
}

func drawHeader(r canvas.Region, v *View, t *Theme) {
	s := v.Snap
	in := Frame(r, t, "", "", t.Host)
	if in.Empty() {
		return
	}
	// The emblem sits at the left. Graphics mode draws it as pixels over
	// the full header height; terminals get a small block-art version.
	ow := 0
	if in.W >= 70 && in.H >= 3 {
		ow = orbCells(in.Square())
		if in.Square() {
			r.Sub(2, 0, ow, r.H).AddOrb(t.Host)
		} else {
			drawOrbCells(in, 0, 0, v.Now, t)
		}
		ow++
	}
	drawTabs(r, v.Page, t, 2+ow)
	now := v.Now
	clock := now.Format("15:04")
	cw := BigWidth(clock, in.Square())
	x := ow
	if in.W >= cw+40+ow && in.H >= 3 {
		x = Big(in, x, 0, clock, func(c int) canvas.Color { return t.Heat(float64(c) / float64(cw) * 0.5) })
		x = in.Text(x+1, 2, now.Format(":05"), t.Dim) + 3
	}

	// Right side: date and at-a-glance chips. Dropped when there is no
	// room, so the identity lines never collide with them.
	date := now.Format("Monday 2 January")
	type chip struct {
		label string
		frac  float64
		text  string
	}
	chips := []chip{{"cpu", s.CPU, Pct(s.CPU)}, {"mem", s.Mem.UsedFrac(), Pct(s.Mem.UsedFrac())}}
	if m := fullestMount(s.Mounts); m != nil {
		chips = append(chips, chip{"disk", m.UsedFrac(), Pct(m.UsedFrac())})
	}
	if hot := hottest(s.Temps); hot != nil {
		chips = append(chips, chip{"temp", tempFrac(*hot), fmt.Sprintf("%.0f°", hot.Celsius)})
	}
	chipsW := 0
	for _, c := range chips {
		chipsW += len([]rune("● "+c.label+" "+c.text)) + 2
	}
	rightW := max(len(date), chipsW)
	id := in.Sub(x, 0, in.W-x, in.H)
	if id.W-rightW-2 >= 50 {
		id = in.Sub(x, 0, id.W-rightW-2, in.H)
		in.Bold(in.W-len(date), 0, date, t.FG)
		cx := in.W - chipsW + 2
		for _, c := range chips {
			col := t.Heat(c.frac)
			cx = in.Set2(cx, 2, '●', col) + 1
			cx = in.Text(cx, 2, c.label+" ", t.Dim)
			cx = in.Bold(cx, 2, c.text, col) + 2
		}
	}

	// Identity lines.
	hn := s.Host.Hostname
	for i, ch := range hn {
		id.SetBold(i, 0, ch, Mix(t.Host, t.CPU, float64(i)/float64(max(len(hn), 1))))
	}
	id.Text(len([]rune(hn))+2, 0, fmt.Sprintf("%s · %s", s.Host.OS, s.Host.Kernel), t.Dim)
	id.Text(0, 1, fmt.Sprintf("%s · %d threads · up %s", s.Host.CPUModel, len(s.Cores), Uptime(s.Host.Uptime)), t.Dim)
	lx := id.Text(0, 2, "load ", t.Dim)
	n := float64(max(len(s.Cores), 1))
	for i, l := range s.Host.Load {
		lx = id.Bold(lx, 2, fmt.Sprintf("%.2f", l), t.Heat(l/n)) + 1
		if i == 0 {
			lx++
		}
	}
	id.Text(lx+1, 2, fmt.Sprintf("· %d tasks · %d running", s.Host.Tasks, s.Host.Running), t.Dim)
}

func drawCPU(r canvas.Region, s *sys.Snapshot, t *Theme) {
	var ghz float64
	for _, f := range s.CoreMHz {
		ghz += f
	}
	sub := fmt.Sprintf("%d threads", len(s.Cores))
	if len(s.CoreMHz) > 0 && ghz > 0 {
		sub += fmt.Sprintf(" · %.1f GHz avg", ghz/float64(len(s.CoreMHz))/1000)
	}
	in := Frame(r, t, "CPU", sub, t.CPU)
	if in.Empty() || len(s.Cores) == 0 {
		return
	}

	// Cores take the left side when wide, the top when narrow.
	coresW, graph := in.W, canvas.Region{}
	if in.W >= 70 {
		coresW = in.W * 45 / 100
		graph = in.Sub(coresW+3, 0, in.W-coresW-3, in.H)
	} else if in.H > len(s.Cores)/2+4 {
		rows := (len(s.Cores) + 1) / 2
		graph = in.Sub(0, rows+1, in.W, in.H-rows-1)
	}
	rows := in.H
	if graph.H > 0 && graph.Y > in.Y {
		rows = graph.Y - in.Y - 1
	}
	cols := (len(s.Cores) + rows - 1) / max(rows, 1)
	colW := coresW / max(cols, 1)
	perCol := (len(s.Cores) + cols - 1) / max(cols, 1)
	showFreq := colW >= 26
	for i, v := range s.Cores {
		x, y := (i/perCol)*colW, i%perCol
		in.Text(x, y, fmt.Sprintf("%02d", i), t.Faint)
		gw := colW - 3 - 5 - 1
		if showFreq {
			gw -= 5
		}
		Gauge(in, x+3, y, gw, v, t, t.heat())
		in.Bold(x+3+gw+1, y, Pct(v), t.Heat(v))
		if showFreq && i < len(s.CoreMHz) {
			in.Text(x+3+gw+6, y, fmt.Sprintf("%3.1fG", s.CoreMHz[i]/1000), t.Dim)
		}
	}

	// Per-core history under the bars when the cores sit on the left.
	if graph.X > in.X {
		hy := perCol + 1
		hm := in.Sub(0, hy+1, coresW, min(in.H-hy-1, (len(s.CoreHist)+1)/2))
		if hm.H >= 2 {
			in.Text(0, hy, "per-core history", t.Faint)
			Heatmap(hm, s.CoreHist, t)
		}
	}

	if graph.H >= 2 {
		x := graph.Bold(0, 0, "total ", t.FG)
		x = graph.Bold(x, 0, strings.TrimSpace(Pct(s.CPU)), t.Heat(s.CPU))
		avg := mean(tail(s.CPUHist, 60))
		graph.TextRight(graph.W, 0, fmt.Sprintf("1m avg %s · peak %s", strings.TrimSpace(Pct(avg)), strings.TrimSpace(Pct(peak(tail(s.CPUHist, 60))))), t.Dim)
		_ = x
		Graph(graph.Sub(0, 1, graph.W, graph.H-1), s.CPUHist, false, t, t.heat())
	}
}

func drawMem(r canvas.Region, s *sys.Snapshot, t *Theme) {
	m := s.Mem
	in := Frame(r, t, "Memory", Bytes(float64(m.Total)), t.Mem)
	if in.Empty() {
		return
	}
	rows := []struct {
		label string
		f     float64
		val   string
	}{
		{"used", m.UsedFrac(), Bytes(float64(m.Used)) + " / " + Bytes(float64(m.Total))},
		{"cache", frac(m.Cached+m.Buffers, m.Total), Bytes(float64(m.Cached + m.Buffers))},
	}
	if m.SwapTotal > 0 {
		rows = append(rows, struct {
			label string
			f     float64
			val   string
		}{"swap", frac(m.SwapUsed, m.SwapTotal), Bytes(float64(m.SwapUsed)) + " / " + Bytes(float64(m.SwapTotal))})
	}
	valW := 14
	for i, row := range rows {
		if i >= in.H {
			return
		}
		in.Text(0, i, row.label, t.Dim)
		gw := in.W - 6 - valW - 6
		Gauge(in, 6, i, gw, row.f, t, t.heat())
		in.Bold(6+gw+1, i, Pct(row.f), t.Heat(row.f))
		in.TextRight(in.W, i, row.val, t.FG)
	}
	if g := in.Sub(0, len(rows)+1, in.W, in.H-len(rows)-1); g.H >= 2 {
		Graph(g, s.MemHist, false, t, t.heat())
	}
}

func drawGPU(r canvas.Region, s *sys.Snapshot, t *Theme) {
	in := Frame(r, t, "GPU", fmt.Sprintf("%d device(s)", len(s.GPUs)), t.GPU)
	if in.Empty() {
		return
	}
	if len(s.GPUs) == 0 {
		in.Text(0, 0, "no amdgpu devices", t.Faint)
		return
	}
	// Discrete cards first.
	order := make([]int, len(s.GPUs))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return s.GPUs[order[a]].Discrete && !s.GPUs[order[b]].Discrete })

	y := 0
	for _, gi := range order {
		g := s.GPUs[gi]
		if y+3 > in.H {
			break
		}
		kind := "iGPU"
		if g.Discrete {
			kind = "dGPU"
		}
		x := in.Bold(0, y, kind, t.GPU)
		x = in.Text(x+1, y, g.Card, t.Faint)
		var meta []string
		if g.TempC > 0 {
			meta = append(meta, fmt.Sprintf("%.0f°C", g.TempC))
		}
		if g.PowerW > 0 {
			meta = append(meta, fmt.Sprintf("%.0fW", g.PowerW))
		}
		if g.SclkMHz > 0 {
			meta = append(meta, fmt.Sprintf("%.0f MHz", g.SclkMHz))
		}
		in.TextRight(in.W, y, strings.Join(meta, " · "), t.Dim)
		_ = x
		gw := in.W - 6 - 14 - 6
		in.Text(0, y+1, "busy", t.Dim)
		Gauge(in, 6, y+1, gw, g.Busy, t, t.heat())
		in.Bold(6+gw+1, y+1, Pct(g.Busy), t.Heat(g.Busy))
		label, used, total := "vram", g.VRAMUsed, g.VRAMTotal
		if !g.Discrete {
			label, used, total = "gtt", g.GTTUsed, g.GTTTotal
		}
		f := frac(used, total)
		in.Text(0, y+2, label, t.Dim)
		Gauge(in, 6, y+2, gw, f, t, t.heat())
		in.Bold(6+gw+1, y+2, Pct(f), t.Heat(f))
		in.TextRight(in.W, y+2, Bytes(float64(used))+" / "+Bytes(float64(total)), t.FG)
		y += 3
		if y < in.H && len(order) > 1 {
			y++ // breathing room between devices
		}
	}
	if g := in.Sub(0, y, in.W, in.H-y); g.H >= 2 && len(s.GPUHist) > 0 {
		g.Text(0, 0, "busy history · "+s.GPUs[order[0]].Card, t.Faint)
		Graph(g.Sub(0, 1, g.W, g.H-1), s.GPUHist[order[0]], false, t, t.heat())
	}
}

func drawNet(r canvas.Region, s *sys.Snapshot, t *Theme) {
	var rx, tx float64
	for _, n := range s.Net {
		rx += n.RxRate
		tx += n.TxRate
	}
	in := Frame(r, t, "Network", "↓ "+Rate(rx)+"  ↑ "+Rate(tx), t.Net)
	if in.Empty() {
		return
	}
	maxIf := min(len(s.Net), max(in.H/3, 1))
	nameW := 0
	for _, n := range s.Net[:maxIf] {
		nameW = max(nameW, min(len([]rune(n.Name)), 12))
	}
	for i := range maxIf {
		n := s.Net[i]
		dot := t.Faint
		if n.Up {
			dot = t.Net
		}
		in.Set(0, i, '●', dot)
		in.Text(2, i, trunc(n.Name, nameW), t.FG)
		x := 2 + nameW + 2
		// The address gets whatever the rates leave; extra ones become +N.
		if room := in.W - 25 - x; room >= 7 && len(n.Addrs) > 0 {
			addr := n.Addrs[0]
			if len(n.Addrs) > 1 {
				addr += fmt.Sprintf(" +%d", len(n.Addrs)-1)
			}
			in.Text(x, i, trunc(addr, room), t.Dim)
		}
		in.TextRight(in.W-12, i, "↓ "+Rate(n.RxRate), t.RX)
		in.TextRight(in.W, i, "↑ "+Rate(n.TxRate), t.TX)
	}
	g := in.Sub(0, maxIf+1, in.W, in.H-maxIf-1)
	if g.H < 2 {
		return
	}
	half := g.H / 2
	up, upPeak := scale(s.NetRx, 1024)
	dn, dnPeak := scale(s.NetTx, 1024)
	top := g.Sub(0, 0, g.W, half)
	bot := g.Sub(0, half, g.W, g.H-half)
	Graph(top, up, false, t, shade(t, t.RX))
	Graph(bot, dn, true, t, shade(t, t.TX))
	top.Text(0, 0, "↓ peak "+Rate(upPeak), t.RX)
	bot.Text(0, bot.H-1, "↑ peak "+Rate(dnPeak), t.TX)
}

func drawDisk(r canvas.Region, s *sys.Snapshot, t *Theme) {
	var rd, wr float64
	for _, d := range s.DiskIO {
		rd += d.ReadRate
		wr += d.WriteRate
	}
	in := Frame(r, t, "Disks", "R "+Rate(rd)+"  W "+Rate(wr), t.Disk)
	if in.Empty() {
		return
	}
	pathW := 12
	valW := 11
	y := 0
	for _, m := range s.Mounts {
		if y >= in.H {
			return
		}
		f := m.UsedFrac()
		in.Text(0, y, trunc(m.Path, pathW), t.FG)
		gw := in.W - pathW - 1 - valW - 6
		Gauge(in, pathW+1, y, gw, f, t, t.heat())
		in.Bold(pathW+1+gw+1, y, Pct(f), t.Heat(f))
		in.TextRight(in.W, y, Bytes(float64(m.Used))+"/"+Bytes(float64(m.Size)), t.Dim)
		y++
	}
	y++
	for _, d := range s.DiskIO {
		if y >= in.H {
			return
		}
		in.Text(0, y, trunc(d.Name, pathW), t.Dim)
		in.TextRight(in.W-12, y, "R "+Rate(d.ReadRate), t.RX)
		in.TextRight(in.W, y, "W "+Rate(d.WriteRate), t.TX)
		y++
	}
	g := in.Sub(0, y+1, in.W, in.H-y-1)
	if g.H < 4 {
		return
	}
	half := g.H / 2
	rv, rp := scale(s.DiskRead, 64*1024)
	wv, wp := scale(s.DiskWrite, 64*1024)
	top, bot := g.Sub(0, 0, g.W, half), g.Sub(0, half, g.W, g.H-half)
	Graph(top, rv, false, t, shade(t, t.RX))
	Graph(bot, wv, true, t, shade(t, t.TX))
	top.Text(0, 0, "R peak "+Rate(rp), t.RX)
	bot.Text(0, bot.H-1, "W peak "+Rate(wp), t.TX)
}

var chipOrder = map[string]int{"cpu": 0, "gpu": 1, "nvme": 2, "dimm": 3, "wifi": 4}

func drawTemp(r canvas.Region, s *sys.Snapshot, t *Theme) {
	sub := ""
	if hot := hottest(s.Temps); hot != nil {
		sub = fmt.Sprintf("max %.0f°C", hot.Celsius)
	}
	in := Frame(r, t, "Thermal", sub, t.Temp)
	if in.Empty() {
		return
	}
	temps := append([]sys.Temp(nil), s.Temps...)
	sort.SliceStable(temps, func(i, j int) bool { return rank(temps[i].Chip) < rank(temps[j].Chip) })
	labelW := 14
	y := 0
	for _, tp := range temps {
		if y >= in.H {
			break
		}
		label := tp.Chip
		if tp.Label != "" {
			label += " " + strings.ToLower(tp.Label)
		}
		in.Text(0, y, trunc(label, labelW), t.Dim)
		f := tempFrac(tp)
		gw := in.W - labelW - 1 - 6
		Gauge(in, labelW+1, y, gw, f, t, t.heat())
		in.Bold(in.W-5, y, fmt.Sprintf("%3.0f°C", tp.Celsius), t.Heat(f))
		y++
	}
	for _, fan := range s.Fans {
		if y >= in.H {
			break
		}
		in.Text(0, y, trunc("fan "+fan.Chip, labelW), t.Dim)
		in.TextRight(in.W, y, fmt.Sprintf("%.0f rpm", fan.RPM), t.FG)
		y++
	}
	// Hottest-sensor history fills what the list leaves.
	if g := in.Sub(0, y+1, in.W, in.H-y-1); g.H >= 3 && len(s.TempHist) > 0 {
		g.Text(0, 0, "hottest sensor", t.Faint)
		norm := make([]float64, len(s.TempHist))
		for i, c := range s.TempHist {
			norm[i] = tempFrac(sys.Temp{Celsius: c})
		}
		Graph(g.Sub(0, 1, g.W, g.H-1), norm, false, t, t.heat())
	}
}

func drawProcs(r canvas.Region, s *sys.Snapshot, t *Theme) {
	in := Frame(r, t, "Processes", "by cpu", t.Proc)
	if in.Empty() {
		return
	}
	pidW, cpuW, memW := 7, 6, 6
	barW := min(10, max(in.W/6, 0))
	nameW := in.W - pidW - cpuW - memW - barW - 4
	in.Text(0, 0, "PID", t.Faint)
	in.Text(pidW+1, 0, "NAME", t.Faint)
	in.TextRight(in.W-memW-1, 0, "CPU", t.Faint)
	in.TextRight(in.W, 0, "MEM", t.Faint)
	for i, p := range s.Procs {
		y := i + 1
		if y >= in.H {
			break
		}
		f := min(p.CPU, 1)
		in.TextRight(pidW, y, fmt.Sprint(p.PID), t.Faint)
		in.Text(pidW+1, y, trunc(p.Name, nameW), t.FG)
		Gauge(in, pidW+1+nameW+1, y, barW, f, t, t.heat())
		in.Bold(in.W-memW-1-cpuW, y, fmt.Sprintf("%5.1f%%", p.CPU*100), t.Heat(f))
		in.TextRight(in.W, y, Bytes(float64(p.RSS)), t.Dim)
	}
}

func rank(chip string) int {
	if r, ok := chipOrder[chip]; ok {
		return r
	}
	return len(chipOrder)
}

func tempFrac(tp sys.Temp) float64 {
	lo, hi := 25.0, 95.0
	if tp.Crit > 60 && tp.Crit < 130 {
		hi = tp.Crit
	}
	return max(0, min(1, (tp.Celsius-lo)/(hi-lo)))
}

func hottest(ts []sys.Temp) *sys.Temp {
	var h *sys.Temp
	for i := range ts {
		if h == nil || tempFrac(ts[i]) > tempFrac(*h) {
			h = &ts[i]
		}
	}
	return h
}

func fullestMount(ms []sys.Mount) *sys.Mount {
	var f *sys.Mount
	for i := range ms {
		if f == nil || ms[i].UsedFrac() > f.UsedFrac() {
			f = &ms[i]
		}
	}
	return f
}

func frac(a, b uint64) float64 {
	if b == 0 {
		return 0
	}
	return max(0, min(1, float64(a)/float64(b)))
}

func tail(v []float64, n int) []float64 { return v[max(len(v)-n, 0):] }

func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

// Uptime formats d as "3d 4h", "4h 12m" or "12m".
func Uptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hrs := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hrs)
	case hrs > 0:
		return fmt.Sprintf("%dh %dm", hrs, mins)
	default:
		return fmt.Sprintf("%dm", mins)
	}
}

// drawTabs writes the page keys into the header's top border, the active
// page lit.
func drawTabs(r canvas.Region, p Page, t *Theme, x0 int) {
	edge := Mix(t.Host, t.BG, 0.55)
	x := r.Text(x0, 0, " ", edge)
	for i, tab := range []struct {
		key, name string
		page      Page
	}{{"d", "DASH", Dash}, {"w", "NEWS", News}, {"n", "NET", Net}} {
		if i > 0 {
			x = r.Text(x, 0, "  ", edge)
		}
		if tab.page == p {
			x = r.Bold(x, 0, tab.key, t.FG)
			x = r.Text(x, 0, " ", edge)
			x = r.Bold(x, 0, tab.name, t.Host)
		} else {
			x = r.Text(x, 0, tab.key, t.Dim)
			x = r.Text(x, 0, " ", edge)
			x = r.Text(x, 0, tab.name, t.Faint)
		}
	}
	r.Text(x, 0, " ", edge)
}
