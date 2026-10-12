package ui

import (
	"fmt"
	"math"

	"nocterm/internal/canvas"
)

// Frame draws a rounded border with the title set into the top edge and an
// optional subtitle on the right, fills the interior, and returns the
// interior with one column of padding on each side.
func Frame(r canvas.Region, t *Theme, title, sub string, accent canvas.Color) canvas.Region {
	if r.W < 2 || r.H < 2 {
		return r.Sub(0, 0, 0, 0)
	}
	edge := Mix(accent, t.BG, 0.55)
	for y := range r.H {
		for x := range r.W {
			if y == 0 || y == r.H-1 || x == 0 || x == r.W-1 {
				continue
			}
			r.SetBG(x, y, t.Panel)
		}
	}
	for x := 1; x < r.W-1; x++ {
		r.Set(x, 0, '─', edge)
		r.Set(x, r.H-1, '─', edge)
	}
	for y := 1; y < r.H-1; y++ {
		r.Set(0, y, '│', edge)
		r.Set(r.W-1, y, '│', edge)
	}
	r.Set(0, 0, '╭', edge)
	r.Set(r.W-1, 0, '╮', edge)
	r.Set(0, r.H-1, '╰', edge)
	r.Set(r.W-1, r.H-1, '╯', edge)

	if title != "" && r.W > 8 {
		x := r.Text(2, 0, " ", edge)
		x = r.Set2(x, 0, '●', accent)
		x = r.Text(x, 0, " ", edge)
		x = r.Bold(x, 0, title, accent)
		x = r.Text(x, 0, " ", edge)
		if sub != "" && r.W-x-len([]rune(sub))-5 > 0 {
			r.Text(r.TextRight(r.W-3, 0, sub, t.Dim)-1, 0, " ", edge)
			r.Text(r.W-3, 0, " ", edge)
		}
	}
	return r.Sub(2, 1, r.W-4, r.H-2)
}

var eighths = []rune{'▏', '▎', '▍', '▌', '▋', '▊', '▉'}

// Gauge draws a w-cell bar filled to f. Each cell is colored by its position
// along the bar, so a fuller bar reaches further into the hot end.
func Gauge(r canvas.Region, x, y, w int, f float64, t *Theme, color func(p float64) canvas.Color) {
	if w <= 0 {
		return
	}
	f = max(0, min(1, f))
	total := int(math.Round(f * float64(w) * 8))
	full, rem := total/8, total%8
	for i := range w {
		c := color((float64(i) + 0.5) / float64(w))
		r.SetBG(x+i, y, t.Track)
		switch {
		case i < full:
			r.Set(x+i, y, '█', c)
		case i == full && rem > 0:
			r.Set(x+i, y, eighths[rem-1], c)
		default:
			r.Set(x+i, y, ' ', c)
		}
	}
}

// braille dot bits, [column][row from top].
var dotBits = [2][4]rune{{0x01, 0x02, 0x04, 0x40}, {0x08, 0x10, 0x20, 0x80}}

// Graph draws vals (each in [0,1]) as a braille area chart filling r, newest
// on the right, two samples per cell. down grows it from the top edge.
// color gets the cell's height in [0,1] (1 = furthest from the base).
func Graph(r canvas.Region, vals []float64, down bool, t *Theme, color func(level float64) canvas.Color) {
	if r.Empty() {
		return
	}
	dotsH := r.H * 4
	start := len(vals) - r.W*2
	height := func(dx int) int {
		i := start + dx
		if i < 0 || i >= len(vals) {
			return -1
		}
		v := max(0, min(1, vals[i]))
		hd := int(math.Round(v * float64(dotsH)))
		if hd == 0 && v > 0.002 {
			hd = 1 // keep any activity visible
		}
		return hd
	}
	for cx := range r.W {
		hs := [2]int{height(cx * 2), height(cx*2 + 1)}
		for cy := range r.H {
			// level counts cells away from the base.
			lvl := r.H - 1 - cy
			if down {
				lvl = cy
			}
			var bits rune
			for col := range 2 {
				for row := range 4 {
					d := lvl*4 + 3 - row // dot index from the base
					if down {
						d = lvl*4 + row
					}
					if d < hs[col] {
						bits |= dotBits[col][row]
					}
				}
			}
			if bits != 0 {
				r.Set(cx, cy, 0x2800+bits, color((float64(lvl)+1)/float64(r.H)))
			} else if hs[0] >= 0 && lvl*2 == r.H-1 && r.H >= 5 {
				r.Set(cx, cy, '┄', t.Track)
			}
		}
	}
}

var sparks = []rune("▁▂▃▄▅▆▇█")

// Spark draws the last w of vals as a one-row sparkline.
func Spark(r canvas.Region, x, y, w int, vals []float64, color func(v float64) canvas.Color) {
	start := len(vals) - w
	for i := range w {
		j := start + i
		if j < 0 {
			continue
		}
		v := max(0, min(1, vals[j]))
		r.Set(x+i, y, sparks[int(v*float64(len(sparks)-1)+0.5)], color(v))
	}
}

// Bytes formats n as a short binary size: 512B 3.4K 12G.
func Bytes(n float64) string {
	units := "BKMGTP"
	i := 0
	for n >= 1024 && i < len(units)-1 {
		n /= 1024
		i++
	}
	switch {
	case i == 0:
		return fmt.Sprintf("%.0fB", n)
	case n < 10:
		return fmt.Sprintf("%.1f%c", n, units[i])
	default:
		return fmt.Sprintf("%.0f%c", n, units[i])
	}
}

// Rate formats bytes/second.
func Rate(n float64) string { return Bytes(n) + "/s" }

// Pct formats a fraction as a right-aligned percentage.
func Pct(f float64) string { return fmt.Sprintf("%3.0f%%", f*100) }

func peak(vals []float64) float64 {
	m := 0.0
	for _, v := range vals {
		m = max(m, v)
	}
	return m
}

// scale divides vals by their peak (with a floor so idle links stay flat).
func scale(vals []float64, floor float64) ([]float64, float64) {
	p := max(peak(vals), floor)
	out := make([]float64, len(vals))
	for i, v := range vals {
		out[i] = v / p
	}
	return out, p
}

func trunc(s string, n int) string {
	rs := []rune(s)
	if n <= 0 {
		return ""
	}
	if len(rs) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return string(rs[:n-1]) + "…"
}

// Heatmap draws one strip per series, newest on the right, two series per
// row using upper half blocks (top series in fg, bottom in bg).
func Heatmap(r canvas.Region, series [][]float64, t *Theme) {
	at := func(s []float64, x int) (canvas.Color, bool) {
		i := len(s) - r.W + x
		if i < 0 {
			return t.Track, false
		}
		return t.Heat(s[i]), true
	}
	for row := range r.H {
		a := row * 2
		if a >= len(series) {
			return
		}
		for x := range r.W {
			top, _ := at(series[a], x)
			bot := t.Panel
			if a+1 < len(series) {
				bot, _ = at(series[a+1], x)
			}
			r.Set(x, row, '▀', top)
			r.SetBG(x, row, bot)
		}
	}
}
