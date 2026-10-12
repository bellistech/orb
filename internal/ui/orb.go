package ui

import (
	"math"
	"time"

	"orb/internal/canvas"
)

// orbCells is how many columns the emblem takes.
func orbCells(square bool) int {
	if square {
		return 5 // 40 px: a 36 px sphere plus halo, the header's full height
	}
	return 6 // 6x6 block pixels in half blocks
}

// OrbMist is the swirl inside the orb at normalized position (dx, dy) in
// the unit disc and time t seconds: 0..1, quantized to a few bands so it
// reads as palette cycling. Shared by both renderers.
func OrbMist(dx, dy, t float64) float64 {
	d := math.Hypot(dx, dy)
	a := math.Atan2(dy, dx)
	v := 0.5 + 0.5*math.Sin(3*a+5*d-2.2*t)*math.Cos(2*a-3*d+1.3*t)
	return math.Floor(v*5) / 4
}

// OrbColor is the orb's color for a mist level at distance d (0 center,
// 1 rim) from the center.
func OrbColor(t *Theme, accent canvas.Color, mist, d float64) canvas.Color {
	deep := Mix(t.BG, accent, 0.45)
	hot := Mix(t.Heat(0.92), canvas.RGB(255, 255, 255), 0.25)
	mist = max(0, min(mist, 1))
	var c canvas.Color
	if mist < 0.5 {
		c = Mix(deep, accent, mist*2)
	} else {
		c = Mix(accent, hot, (mist-0.5)*2)
	}
	c = Mix(c, canvas.RGB(255, 255, 255), max(0, 0.35-d)*0.9)  // glowing core
	return Mix(c, Mix(t.BG, accent, 0.2), math.Pow(d, 3)*0.75) // darker limb
}

var orbMask = [6]string{".####.", "######", "######", "######", "######", ".####."}

// drawOrbCells draws the terminal version: a 6x6 pixel sphere in half
// blocks, its mist moving once per redraw.
func drawOrbCells(r canvas.Region, x, y int, now time.Time, t *Theme) {
	ts := float64(now.UnixMilli()%1_000_000) / 1000
	px := func(cx, py int) (canvas.Color, bool) {
		if orbMask[py][cx] != '#' {
			return canvas.Color{}, false
		}
		dx, dy := (float64(cx)-2.5)/3, (float64(py)-2.5)/3
		if cx == 1 && py == 1 {
			return canvas.RGB(255, 255, 255), true // highlight
		}
		return OrbColor(t, t.Host, OrbMist(dx, dy, ts), math.Hypot(dx, dy)), true
	}
	for cy := range 3 {
		for cx := range 6 {
			top, okT := px(cx, cy*2)
			bot, okB := px(cx, cy*2+1)
			switch {
			case okT && okB:
				r.Set(x+cx, y+cy, '▀', top)
				r.SetBG(x+cx, y+cy, bot)
			case okT:
				r.Set(x+cx, y+cy, '▀', top)
			case okB:
				r.Set(x+cx, y+cy, '▄', bot)
			}
		}
	}
}
