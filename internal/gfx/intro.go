package gfx

import (
	"math"

	"orb/internal/canvas"
	"orb/internal/ui"
)

// Studio is the splash screen's wordmark: the GitHub org orb lives under.
const Studio = "bellistech"

// Boot intro timing, in seconds. The title card holds until the dashboard
// has a sample with rates (one sample interval after start), at most
// titleWait longer, so the dashboard is ready when the intro ends.
const (
	splashLen = 1.7 // studio splash: fade in, a light sweeps it, a glint, fade out
	titleMin  = 2.1 // the title card shows at least this long
	titleWait = 4.0
	introFade = 0.3 // fade to the dashboard
)

// Intro is the boot sequence graphics mode plays on start (Stevie,
// 2026-10-23: a SNES-style boot animation): a studio splash, then a title card
// with the orb, ORB, a subtitle and a blinking PRESS START. Each frame is a
// pure function of the time since start (and when the fade out began), so
// tests run it on a virtual clock.
type Intro struct {
	Sub string // title card subtitle: version and hostname

	end       float64 // when the fade out began; 0 until then
	cur, next *Frame  // the frame shown, and the one being drawn
	stars     []star
	orbs      Renderer // orb palette cache
}

// Over reports whether the intro is done at el seconds. ready says the
// dashboard has a sample with rates to show.
func (in *Intro) Over(el float64, ready bool) bool {
	if in.end == 0 && el >= splashLen+titleMin && (ready || el >= splashLen+titleMin+titleWait) {
		in.end = el
	}
	return in.end != 0 && el >= in.end+introFade
}

// Frame is the frame the last Draw composed. Callers must not modify it.
func (in *Intro) Frame() *Frame { return in.cur }

// Reset makes the next Draw report every row, as after a console switch.
func (in *Intro) Reset() { in.cur = nil }

// Draw composes the w x h frame at el seconds and returns the rows that
// changed since the last Draw (all of them the first time).
func (in *Intro) Draw(w, h int, t *ui.Theme, el float64) []Span {
	if in.next == nil || in.next.W != w || in.next.H != h {
		in.next, in.cur = NewFrame(w, h), nil
		in.stars = in.stars[:0]
		for y := range h {
			for x := range w {
				if hv := hash(uint32(x), uint32(y)); hv%997 == 0 {
					in.stars = append(in.stars, star{x, y, hv})
				}
			}
		}
	}
	f := in.next
	bg := rgb(t.BG)
	for i := range f.Pix {
		f.Pix[i] = bg
	}
	if el < splashLen {
		drawSplash(f, t, el)
	} else {
		in.drawTitle(f, t, el-splashLen)
	}
	if in.end != 0 {
		fadeTo(f, bg, (el-in.end)/introFade)
	}

	// Only rows that differ from the frame on screen go out.
	var spans []Span
	for y := range h {
		row := f.Pix[y*w : (y+1)*w]
		if in.cur == nil {
			spans = append(spans, Span{Y: y, X0: 0, X1: w})
			continue
		}
		old := in.cur.Pix[y*w : (y+1)*w]
		x0, x1 := 0, w
		for x0 < w && row[x0] == old[x0] {
			x0++
		}
		if x0 == w {
			continue
		}
		for row[x1-1] == old[x1-1] {
			x1--
		}
		spans = append(spans, Span{Y: y, X0: x0, X1: x1})
	}
	if in.cur == nil {
		in.cur = NewFrame(w, h)
	}
	in.cur, in.next = in.next, in.cur
	return spans
}

// drawSplash is the studio splash at s seconds: the wordmark fades in over
// a line drawn out from its middle, a light sweeps across it, a glint
// blooms on its last letter, and it fades out.
func drawSplash(f *Frame, t *ui.Theme, s float64) {
	bg := rgb(t.BG)
	k := max(1, min(f.W*9/20/textW(Studio, 1), f.H/48)) // font pixel size
	tw, th := textW(Studio, k), 7*k
	x0, y0 := (f.W-tw)/2, (f.H-th)/2-k
	alpha := min(1, s/0.35, (splashLen-s)/0.35)
	if alpha <= 0 {
		return
	}
	// Each letter a step along the header's hostname gradient.
	n := len([]rune(Studio))
	cols := make([]uint32, n)
	for i := range cols {
		cols[i] = rgb(ui.Mix(t.Host, t.CPU, float64(i)/float64(max(n-1, 1))))
	}
	sweep := (s - 0.55) / 0.5 // 0..1 while the light crosses
	shadow := lerpRGB(bg, rgb(ui.Mix(t.BG, canvas.RGB(0, 0, 0), 0.6)), alpha)
	textPx(f, x0+k/2+1, y0+k/2+1, Studio, k, func(int, int) uint32 { return shadow })
	textPx(f, x0, y0, Studio, k, func(x, y int) uint32 {
		c := cols[min((x-x0)/(6*k), n-1)]
		if sweep > 0 && sweep < 1 {
			d := float64(x-x0+(y-y0)/2)/float64(tw) - (sweep*1.4 - 0.2)
			c = lerpRGB(c, 0xFFFFFF, max(0, 1-math.Abs(d)*12)*0.85)
		}
		return lerpRGB(bg, c, alpha)
	})
	// The line under the wordmark, drawn out from the middle.
	if grow := min(1, max(0, (s-0.15)/0.45)); grow > 0 {
		half := int(float64(tw/2) * (1 - (1-grow)*(1-grow)))
		ly := y0 + th + 2*k
		for x := f.W/2 - half; x < f.W/2+half; x++ {
			edge := 1 - math.Abs(float64(x-f.W/2))/float64(tw/2+1)
			c := lerpRGB(bg, rgb(t.Host), alpha*min(1, edge*3))
			for y := ly; y < ly+max(1, k/3); y++ {
				f.set(x, y, c)
			}
		}
	}
	// The glint on the last letter's top right.
	if u := (s - 0.95) / 0.5; u > 0 && u < 1 {
		glint(f, x0+tw, y0, 2*k, math.Sin(math.Pi*u)*alpha, 0xFFFFFF)
	}
}

// drawTitle is the title card s seconds in: stars, the orb growing out of
// nothing, a flash, ORB in big chrome letters, the subtitle typed out, and
// PRESS START blinking.
func (in *Intro) drawTitle(f *Frame, t *ui.Theme, s float64) {
	bg := rgb(t.BG)
	// Stars fade in and twinkle, as behind the dashboard.
	var lvls [starLevels]uint32
	for i := range lvls {
		lvls[i] = lerpRGB(bg, rgb(ui.Mix(t.BG, t.FG, 0.15+0.45*float64(i)/(starLevels-1))), min(1, s/0.5))
	}
	for _, st := range in.stars {
		v := 0.5 + 0.5*math.Sin(s*(0.6+float64(st.h%7)/5)+float64(st.h%628)/100)
		f.Pix[st.y*f.W+st.x] = lvls[int(v*(starLevels-1))]
	}

	// The orb grows in over the first half second, easing out.
	cx, cy := f.W/2, f.H*30/100
	R := float64(f.H) * 0.16
	grow := min(1, s/0.5)
	r := int(R*(1-(1-grow)*(1-grow))) + 2
	pal := in.orbs.palette(t, t.Host)
	drawOrb(f, rect{cx - r, cy - r, cx + r, cy + r}, pal, rgb(t.Host), Anim{T: s + 1})
	if s < 0.5 {
		return
	}
	// Glints around the orb, staggered.
	core := rgb(ui.Mix(t.Host, canvas.RGB(255, 255, 255), 0.7))
	for i := range 4 {
		u := math.Mod(s-0.6+float64(i)*0.55, 2.2) / 0.8
		if s < 0.6 || u >= 1 {
			continue
		}
		a := float64(i)*2.4 + 0.7
		gx, gy := cx+int((R+6)*math.Cos(a)), cy+int((R+6)*math.Sin(a))
		glint(f, gx, gy, max(3, int(R/12)), math.Sin(math.Pi*u), core)
	}

	// ORB: dark outline and drop shadow, a chrome gradient top to bottom,
	// and a light sweeping across every 2.5 s.
	const title = "ORB"
	k := max(2, min(f.W/4/textW(title, 1), f.H*17/100/7))
	tw, th := textW(title, k), 7*k
	tx, ty := (f.W-tw)/2, f.H*52/100
	outline := rgb(ui.Mix(t.BG, canvas.RGB(0, 0, 0), 0.6))
	o := max(1, k/5)
	textPx(f, tx+o*2, ty+o*2, title, k, func(int, int) uint32 { return outline })
	for _, d := range [][2]int{{-o, 0}, {o, 0}, {0, -o}, {0, o}} {
		textPx(f, tx+d[0], ty+d[1], title, k, func(int, int) uint32 { return outline })
	}
	top, mid := ui.Mix(t.FG, canvas.RGB(255, 255, 255), 0.6), t.Host
	low, hot := ui.Mix(t.Host, t.BG, 0.35), t.Heat(0.85)
	grad := make([]uint32, Tile*k) // descender rows too
	for y := range grad {
		v := min(1, float64(y)/float64(th))
		switch {
		case v < 0.45:
			grad[y] = rgb(ui.Mix(top, mid, v/0.45))
		case v < 0.55:
			grad[y] = rgb(ui.Mix(mid, low, (v-0.45)/0.1))
		default:
			grad[y] = rgb(ui.Mix(low, hot, (v-0.55)/0.45))
		}
	}
	sweep := math.Mod(s-0.9, 2.5) / 0.7
	textPx(f, tx, ty, title, k, func(x, y int) uint32 {
		px := grad[y-ty]
		if s > 0.9 && sweep < 1 {
			d := float64(x-tx+(y-ty)/2)/float64(tw) - (sweep*1.6 - 0.3)
			px = lerpRGB(px, 0xFFFFFF, max(0, 1-math.Abs(d)*8)*0.8)
		}
		return px
	})

	// The subtitle types out, letters glowing as they land.
	sk := max(1, f.H/240)
	sub := []rune(in.Sub)
	sub = sub[:min(len(sub), max(0, (f.W-4*sk)/(6*sk)))]
	if n := min(len(sub), int((s-0.7)*30)); n > 0 {
		sx, sy := (f.W-textW(string(sub), sk))/2, ty+th+f.H*6/100
		for i, ch := range sub[:n] {
			age := (s - 0.7) - float64(i)/30
			c := lerpRGB(rgb(t.Dim), 0xFFFFFF, max(0, 1-age/0.35))
			textPx(f, sx+i*6*sk+sk/2+1, sy+sk/2+1, string(ch), sk, func(int, int) uint32 { return outline })
			textPx(f, sx+i*6*sk, sy, string(ch), sk, func(int, int) uint32 { return c })
		}
	}

	// PRESS START, blinking.
	if s > 1.2 && math.Mod(s-1.2, 1) < 0.6 {
		const ps = "PRESS START"
		px, py := (f.W-textW(ps, sk))/2, f.H*86/100
		textPx(f, px+sk/2+1, py+sk/2+1, ps, sk, func(int, int) uint32 { return outline })
		textPx(f, px, py, ps, sk, func(int, int) uint32 { return rgb(t.FG) })
	}

	// The flash as the title lands.
	if s < 0.8 {
		flash := 0.55 * (1 - (s-0.5)/0.3)
		for i, v := range f.Pix {
			f.Pix[i] = lerpRGB(v, 0xFFFFFF, flash)
		}
	}
}

// textW is the pixel width of s in the font scaled by k.
func textW(s string, k int) int { return max(0, (6*len([]rune(s))-1)*k) }

// textPx draws s in the font at (x, y), each font pixel a k x k block,
// every pixel colored by color (a gradient per scanline, as the consoles
// did with mid-frame palette writes).
func textPx(f *Frame, x, y int, s string, k int, color func(x, y int) uint32) {
	for i, ch := range []rune(s) {
		g, ok := font[ch]
		if !ok {
			g = font['?']
		}
		gx := x + i*6*k
		for row, bits := range g {
			for col := 1; col <= 5; col++ {
				if bits&(0x80>>col) == 0 {
					continue
				}
				bx, by := gx+(col-1)*k, y+row*k
				for py := by; py < by+k; py++ {
					for px := bx; px < bx+k; px++ {
						f.set(px, py, color(px, py))
					}
				}
			}
		}
	}
}

// fadeTo blends every pixel of f toward v by k (0..1).
func fadeTo(f *Frame, v uint32, k float64) {
	if k <= 0 {
		return
	}
	for i, p := range f.Pix {
		if p != v {
			f.Pix[i] = lerpRGB(p, v, k)
		}
	}
}
