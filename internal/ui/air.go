package ui

import (
	"fmt"
	"sort"
	"time"

	"orb/internal/canvas"
	"orb/internal/radio"
	"orb/internal/sys"
)

// The airwaves page: what the host's radios hear. Nearby Wi-Fi networks
// from their beacons, how crowded each channel is, and the radios
// themselves. orb only listens: it never joins a network or sends a probe.

func airGrid(w int) []row {
	switch {
	case w >= 180:
		return []row{
			{62, []slot{{drawAirNets, 64, nil}, {nil, 36, []slot{{drawChan24, 50, nil}, {drawChan5, 50, nil}}}}},
			{38, []slot{{drawRadios, 58, nil}, {drawAirMix, 42, nil}}},
		}
	case w >= 100:
		return []row{
			{46, []slot{{drawAirNets, 1, nil}}},
			{24, []slot{{drawChan24, 50, nil}, {drawChan5, 50, nil}}},
			{30, []slot{{drawRadios, 58, nil}, {drawAirMix, 42, nil}}},
		}
	default:
		return []row{
			{40, []slot{{drawAirNets, 1, nil}}},
			{14, []slot{{drawChan24, 1, nil}}},
			{14, []slot{{drawChan5, 1, nil}}},
			{18, []slot{{drawRadios, 1, nil}}},
			{14, []slot{{drawAirMix, 1, nil}}},
		}
	}
}

func airOrWait(in canvas.Region, v *View, t *Theme) *sys.Radio {
	if v.Snap.Radio == nil {
		in.Text(0, 0, "listening…", t.Dim)
	}
	return v.Snap.Radio
}

// sigFrac places a signal on the cool-to-hot ramp: -90 dBm cold, -30 hot.
func sigFrac(dbm float64) float64 {
	if dbm == 0 {
		return 0
	}
	return max(0, min(1, (dbm+90)/60))
}

// secColor marks weak protection hot: anyone can read an open network.
func secColor(t *Theme, sec string) canvas.Color {
	switch sec {
	case "open", "WEP":
		return t.Heat(1)
	case "WPA", "OWE":
		return t.Heat(0.6)
	case "WPA3", "WPA3-EAP":
		return t.Heat(0)
	}
	return t.Dim
}

// ago is a short age: now, 12s, 4m, 2h.
func ago(now, then time.Time) string {
	d := now.Sub(then)
	switch {
	case d < 5*time.Second:
		return "now"
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}

func drawAirNets(r canvas.Region, v *View, t *Theme) {
	rv := v.Snap.Radio
	sub := ""
	if rv != nil {
		sub = plural(len(rv.APs), "network") + " · last " + fmt.Sprint(int(sys.RadioKeep.Minutes())) + " min"
	}
	in := Frame(r, t, "Wi-Fi networks", sub, t.Net)
	if in.Empty() || airOrWait(in, v, t) == nil {
		return
	}
	if len(rv.APs) == 0 {
		in.Text(0, 0, airHint(rv), t.Dim)
		return
	}
	now := v.Snap.Time
	// Columns, dropped right to left as the panel narrows.
	gw, dbW, chW, bandW, secW, macW, ageW := 8, 7, 4, 5, 9, 17, 6
	showMAC := in.W >= 70
	showSec := in.W >= 48
	showBand := in.W >= 40
	fixed := gw + 1 + dbW + 1 + chW + ageW + 1
	if showBand {
		fixed += bandW
	}
	if showSec {
		fixed += secW + 1
	}
	if showMAC {
		fixed += macW + 1
	}
	ssidW := max(in.W-fixed, 4)
	ssidW = min(ssidW, 32)
	x := 0
	in.Text(x, 0, "SIGNAL", t.Faint)
	x += gw + 1 + dbW + 1
	in.Text(x, 0, "NETWORK", t.Faint)
	x += ssidW + 1
	in.TextRight(x+chW-1, 0, "CH", t.Faint)
	x += chW
	if showBand {
		in.TextRight(x+bandW-1, 0, "GHz", t.Faint)
		x += bandW
	}
	if showSec {
		x++
		in.Text(x, 0, "SECURITY", t.Faint)
		x += secW
	}
	if showMAC {
		x++
		in.Text(x, 0, "BSSID", t.Faint)
	}
	in.TextRight(in.W, 0, "HEARD", t.Faint)
	for i, ap := range rv.APs {
		y := i + 1
		if y >= in.H {
			break
		}
		if y == in.H-1 && i < len(rv.APs)-1 {
			in.Text(0, y, fmt.Sprintf("+%d more", len(rv.APs)-i), t.Faint)
			break
		}
		f := sigFrac(ap.Signal)
		stale := now.Sub(ap.Seen) > time.Minute
		Gauge(in, 0, y, gw, f, t, t.heat())
		db := "  -"
		if ap.Signal != 0 {
			db = fmt.Sprintf("%.0f dBm", ap.Signal)
		}
		in.TextRight(gw+1+dbW, y, db, t.Heat(f))
		x := gw + 1 + dbW + 1
		name, nc := ap.SSID, t.FG
		if ap.Hidden {
			name, nc = "(hidden)", t.Faint
		} else if stale {
			nc = t.Dim
		}
		if ap.Joined {
			in.Bold(x, y, trunc(name, ssidW), t.Host)
		} else {
			in.Text(x, y, trunc(name, ssidW), nc)
		}
		x += ssidW + 1
		ch := "-"
		if c := radio.Channel(ap.FreqMHz); c > 0 {
			ch = fmt.Sprint(c)
		}
		in.TextRight(x+chW-1, y, ch, t.FG)
		x += chW
		if showBand {
			in.TextRight(x+bandW-1, y, radio.Band(ap.FreqMHz), t.Dim)
			x += bandW
		}
		if showSec {
			x++
			in.Text(x, y, trunc(ap.Security, secW), secColor(t, ap.Security))
			x += secW
		}
		if showMAC {
			x++
			in.Text(x, y, radio.MAC(ap.BSSID), t.Dim)
		}
		ac := t.Dim
		if stale {
			ac = t.Faint
		}
		in.TextRight(in.W, y, ago(now, ap.Seen), ac)
	}
}

// airHint says why the list is empty.
func airHint(rv *sys.Radio) string {
	switch {
	case rv.Err != "":
		return rv.Err
	case len(rv.Ifaces) == 0:
		return "no Wi-Fi radio"
	}
	up := false
	for _, i := range rv.Ifaces {
		up = up || i.Up
	}
	switch {
	case !up:
		return "every radio is down: nothing to listen with"
	case !rv.Scan.On:
		return "nothing heard yet: run as root with radio_scan = 60s to listen"
	}
	return "listening: nothing heard yet"
}

func drawChan24(r canvas.Region, v *View, t *Theme) {
	chans := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}
	drawChannels(r, v, t, "2.4 GHz", chans, func(mhz int) bool { return radio.Band(mhz) == "2.4" })
}

func drawChan5(r canvas.Region, v *View, t *Theme) {
	// 5 and 6 GHz have dozens of channels: show the ones heard.
	var chans []int
	if rv := v.Snap.Radio; rv != nil {
		seen := map[int]bool{}
		for _, ap := range rv.APs {
			if b := radio.Band(ap.FreqMHz); b == "5" || b == "6" {
				if c := radio.Channel(ap.FreqMHz); c > 0 && !seen[c] {
					seen[c] = true
					chans = append(chans, c)
				}
			}
		}
		sort.Ints(chans)
	}
	drawChannels(r, v, t, "5 & 6 GHz", chans, func(mhz int) bool { b := radio.Band(mhz); return b == "5" || b == "6" })
}

var vEighths = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇'}

// drawChannels draws one column per channel: its height the number of
// networks on it, its color the strongest of them.
func drawChannels(r canvas.Region, v *View, t *Theme, title string, chans []int, in func(int) bool) {
	rv := v.Snap.Radio
	count := map[int]int{}
	best := map[int]float64{}
	total := 0
	if rv != nil {
		for _, ap := range rv.APs {
			if !in(ap.FreqMHz) {
				continue
			}
			c := radio.Channel(ap.FreqMHz)
			count[c]++
			total++
			if f := sigFrac(ap.Signal); f > best[c] {
				best[c] = f
			}
		}
	}
	box := Frame(r, t, title, plural(total, "network"), t.CPU)
	if box.Empty() || airOrWait(box, v, t) == nil {
		return
	}
	if len(chans) == 0 {
		box.Text(0, 0, "none heard", t.Dim)
		return
	}
	top := 3 // one network is a third of the height, not all of it
	for _, c := range chans {
		top = max(top, count[c])
	}
	colW := max(box.W/len(chans), 1)
	shown := min(len(chans), box.W/colW)
	barH := box.H - 1
	for i, c := range chans[:shown] {
		x := i * colW
		label := fmt.Sprint(c)
		if len(label) <= colW {
			box.Text(x+(colW-len(label))/2, box.H-1, label, t.Dim)
		}
		if barH <= 0 || count[c] == 0 {
			continue
		}
		col := t.Heat(best[c])
		eighthsH := count[c] * barH * 8 / top
		bw := max(colW-1, 1)
		for k := 0; k < barH; k++ {
			y := box.H - 2 - k
			fill := eighthsH - k*8
			var ch rune
			switch {
			case fill >= 8:
				ch = '█'
			case fill > 0:
				ch = vEighths[fill-1]
			default:
				continue
			}
			for dx := range bw {
				box.Set(x+dx, y, ch, col)
			}
		}
		if n := fmt.Sprint(count[c]); barH > 1 && len(n) <= colW {
			ny := box.H - 2 - min(barH-1, (eighthsH+7)/8)
			box.Text(x+(colW-len(n))/2, ny, n, t.FG)
		}
	}
}

func drawRadios(r canvas.Region, v *View, t *Theme) {
	rv := v.Snap.Radio
	sub := ""
	if rv != nil {
		sub = plural(len(rv.Ifaces), "radio")
		if rv.Scan.On {
			sub += " · passive scan every " + shortDur(rv.Scan.Every)
		} else {
			sub += " · heard only"
		}
	}
	in := Frame(r, t, "Radios", sub, t.Mem)
	if in.Empty() || airOrWait(in, v, t) == nil {
		return
	}
	if len(rv.Ifaces) == 0 {
		in.Text(0, 0, airHint(rv), t.Dim)
		return
	}
	now := v.Snap.Time
	joined := map[string]radio.AP{}
	for _, ap := range rv.APs {
		if ap.Joined {
			joined[ap.Iface] = ap
		}
	}
	y := 0
	for _, i := range rv.Ifaces {
		if y >= in.H {
			break
		}
		dot, dc := "●", t.Heat(0)
		if !i.Up {
			dot, dc = "○", t.Faint
		}
		x := in.Text(0, y, dot, dc)
		x = in.Bold(x+1, y, i.Name, t.Host)
		x = in.Text(x+1, y, fmt.Sprintf("phy%d %s", i.Phy, i.Type), t.Dim)
		if in.W-x > 19 {
			in.TextRight(in.W, y, radio.MAC(i.MAC), t.Faint)
		}
		y++
		if y >= in.H {
			break
		}
		var state string
		sc := t.FG
		switch ap, ok := joined[i.Name]; {
		case !i.Up:
			state, sc = "down", t.Faint
		case ok:
			name := ap.SSID
			if ap.Hidden {
				name = "(hidden)"
			}
			state = fmt.Sprintf("joined %s · ch %d · %s GHz", name, radio.Channel(ap.FreqMHz), radio.Band(ap.FreqMHz))
		case i.FreqMHz > 0:
			state = fmt.Sprintf("in use · ch %d", radio.Channel(i.FreqMHz))
		default:
			state = "up · listening only"
		}
		in.Text(2, y, trunc(state, in.W-2), sc)
		y++
		if y >= in.H {
			break
		}
		scan, scc := "no scans asked (heard only)", t.Faint
		switch {
		case rv.Scan.On && i.Scan.At.IsZero():
			scan, scc = "passive scan: waiting for the first", t.Dim
		case rv.Scan.On && i.Scan.Err == "":
			scan, scc = "passive scan "+ago(now, i.Scan.At)+" ago", t.Dim
		case rv.Scan.On:
			scan, scc = "passive scan "+ago(now, i.Scan.At)+" ago: "+i.Scan.Err, t.Heat(0.8)
		}
		in.Text(2, y, trunc(scan, in.W-2), scc)
		y += 2
	}
}

// shortDur prints 60s as 1m, 90s as 90s, 1h as 1h.
func shortDur(d time.Duration) string {
	switch {
	case d >= time.Hour && d%time.Hour == 0:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d >= time.Minute && d%time.Minute == 0:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

// drawAirMix counts networks by protection and band.
func drawAirMix(r canvas.Region, v *View, t *Theme) {
	rv := v.Snap.Radio
	in := Frame(r, t, "On the air", "", t.Disk)
	if in.Empty() || airOrWait(in, v, t) == nil {
		return
	}
	secs := []string{"open", "WEP", "WPA", "WPA2", "WPA2/3", "WPA3", "OWE", "WPA2-EAP", "WPA3-EAP"}
	n := map[string]int{}
	bands := map[string]int{}
	hidden := 0
	for _, ap := range rv.APs {
		n[ap.Security]++
		bands[radio.Band(ap.FreqMHz)]++
		if ap.Hidden {
			hidden++
		}
	}
	total := max(len(rv.APs), 1)
	labelW, cntW := 9, 4
	barW := max(in.W-labelW-cntW-2, 0)
	y := 0
	for _, s := range secs {
		if n[s] == 0 {
			continue
		}
		if y >= in.H {
			return
		}
		in.Text(0, y, s, secColor(t, s))
		Gauge(in, labelW+1, y, barW, float64(n[s])/float64(total), t, func(float64) canvas.Color { return secColor(t, s) })
		in.TextRight(in.W, y, fmt.Sprint(n[s]), t.FG)
		y++
	}
	if y == 0 {
		in.Text(0, 0, "nothing heard", t.Dim)
		return
	}
	y++
	if y < in.H {
		x := 0
		for _, b := range []string{"2.4", "5", "6"} {
			if bands[b] > 0 {
				x = in.Text(x, y, fmt.Sprintf("%s GHz ", b), t.Dim)
				x = in.Bold(x, y, fmt.Sprint(bands[b]), t.FG)
				x = in.Text(x, y, "   ", t.Dim)
			}
		}
		if hidden > 0 {
			x = in.Text(x, y, "hidden ", t.Dim)
			in.Bold(x, y, fmt.Sprint(hidden), t.FG)
		}
	}
}
