package gfx

import (
	"math"
	"time"

	"orb/internal/canvas"
	"orb/internal/feeds"
	"orb/internal/ui"
)

// Sky is what the backdrop behind the windows shows: the time of day and
// the current weather. The zero value is the plain night starfield.
type Sky struct {
	Now  time.Time  // local wall clock; zero means no day or night
	Kind feeds.Kind // current conditions; Unknown is a clear sky
}

// NewSky returns the sky at now under the weather feed's current
// conditions, if it has any.
func NewSky(now time.Time, fv feeds.View) Sky {
	s := Sky{Now: now}
	if fv.Weather != nil {
		s.Kind = fv.Weather.Now.Kind
	}
	return s
}

const (
	// skyLatitude stands in for the place: orb has no location and looks
	// none up, so sunrise and sunset are those of a mid-northern latitude.
	skyLatitude = 35.0
	skySteps    = 48 // light, glow and overcast steps; the sky repaints on a change
	bodyR       = 6  // sun and moon radius, logical pixels
	haloR       = 48 // sun glow radius: light spills through the gaps
)

type phase uint8

const (
	night phase = iota
	dawn
	day
	dusk
)

func (p phase) String() string { return [...]string{"night", "dawn", "day", "dusk"}[p] }

// clockHours is t's time of day in hours on its own clock.
func clockHours(t time.Time) float64 {
	return float64(t.Hour()) + float64(t.Minute())/60 + float64(t.Second())/3600
}

// sunTimes approximates sunrise and sunset on t's date, in hours on t's
// clock: day length from the solar declination at skyLatitude, centered
// on noon standard time (13:00 under daylight saving time).
func sunTimes(t time.Time) (rise, set float64) {
	decl := -23.44 * math.Pi / 180 * math.Cos(2*math.Pi*(float64(t.YearDay())+10)/365)
	x := -math.Tan(skyLatitude*math.Pi/180) * math.Tan(decl)
	half := math.Acos(max(-1, min(1, x))) * 12 / math.Pi
	noon := 12.0
	if t.IsDST() {
		noon = 13
	}
	return noon - half, noon + half
}

func smoothstep(a, b, x float64) float64 {
	u := max(0, min(1, (x-a)/(b-a)))
	return u * u * (3 - 2*u)
}

// daylight is how light the sky is at t (0 night, 1 day) and how much
// sunrise or sunset glow it holds (0..1).
func daylight(t time.Time) (light, glow float64) {
	rise, set := sunTimes(t)
	h := clockHours(t)
	light = smoothstep(rise-0.75, rise+0.5, h) * (1 - smoothstep(set-0.5, set+0.75, h))
	glow = max(0, 1-math.Abs(h-rise), 1-math.Abs(h-set))
	return light, glow
}

// phaseAt names the part of the day the sky shows at t.
func phaseAt(t time.Time) phase {
	light, _ := daylight(t)
	rise, set := sunTimes(t)
	switch {
	case light >= 0.98:
		return day
	case light <= 0.02:
		return night
	case clockHours(t) < (rise+set)/2:
		return dawn
	}
	return dusk
}

// bodyAt says how far along its arc the sun is at t (0 rising, 0.5
// highest, 1 setting), or, between sunset and sunrise, the moon.
func bodyAt(t time.Time) (frac float64, sun bool) {
	rise, set := sunTimes(t)
	h := clockHours(t)
	if h >= rise && h <= set {
		return (h - rise) / (set - rise), true
	}
	since := h - set
	if since < 0 {
		since += 24
	}
	return since / (24 - (set - rise)), false
}

// arc places a body frac along its path across a w x h frame: up from the
// bottom left corner, over row top at the center, down past the bottom
// right.
func arc(frac float64, w, h, top int) (x, y int) {
	bottom := float64(h + bodyR)
	x = int(math.Round(float64(bodyR) + frac*float64(w-2*bodyR)))
	y = int(math.Round(bottom - (bottom-float64(top))*math.Sin(math.Pi*frac)))
	return x, y
}

// moonPhase is the moon's age at t as a fraction of the synodic month:
// 0 new, 0.25 first quarter, 0.5 full, 0.75 last quarter.
func moonPhase(t time.Time) float64 {
	const synodic = 29.530588853                                // days
	ref := time.Date(2000, 1, 6, 18, 14, 0, 0, time.UTC).Unix() // a new moon
	d := float64(t.Unix()-ref) / 86400 / synodic
	return d - math.Floor(d)
}

// fx is how the backdrop shows a weather condition.
type fx struct {
	clouds    float64 // cloud cover, 0..1
	overcast  float64 // how much the sky greys and hides sun, moon and stars
	rain      bool
	snow      bool
	fog       bool
	lightning bool
}

func weatherFX(k feeds.Kind) fx {
	switch k {
	case feeds.PartlyCloudy:
		return fx{clouds: 0.35, overcast: 0.1}
	case feeds.Cloudy:
		return fx{clouds: 1, overcast: 0.6}
	case feeds.Fog:
		return fx{clouds: 0.2, overcast: 0.5, fog: true}
	case feeds.Rain:
		return fx{clouds: 0.8, overcast: 0.7, rain: true}
	case feeds.Storm:
		return fx{clouds: 1, overcast: 0.9, rain: true, lightning: true}
	case feeds.Snow:
		return fx{clouds: 0.7, overcast: 0.6, snow: true}
	}
	return fx{}
}

// skyState is what the cached sky in the base layer depends on, quantized
// so that it changes every minute or so, not every frame.
type skyState struct {
	on                bool // false: the plain starfield
	light, glow, cast uint8
	fog               bool
	sun               bool
	bx, by            int   // sun or moon center
	moon              uint8 // phase in 32 steps
}

// quant quantizes v in [0,1] to skySteps steps.
func quant(v float64) uint8 { return uint8(math.Round(max(0, min(1, v)) * skySteps)) }

// skyStateOf is the sky s over a w x h frame whose highest gap between
// windows is centered on row top.
func skyStateOf(s Sky, w, h, top int) skyState {
	if s.Now.IsZero() {
		return skyState{}
	}
	light, glow := daylight(s.Now)
	wx := weatherFX(s.Kind)
	frac, sun := bodyAt(s.Now)
	st := skyState{on: true, light: quant(light), glow: quant(glow), cast: quant(wx.overcast), fog: wx.fog, sun: sun,
		moon: uint8(math.Round(moonPhase(s.Now)*32)) % 32}
	st.bx, st.by = arc(frac, w, h, top)
	return st
}

// sameGradient reports whether a and b differ only in where the body is.
func sameGradient(a, b skyState) bool {
	a.sun, a.bx, a.by, a.moon = b.sun, b.bx, b.by, b.moon
	return a == b
}

func (s skyState) bodyRect() rect {
	if !s.on {
		return rect{}
	}
	return rect{s.bx - haloR, s.by - haloR, s.bx + haloR + 1, s.by + haloR + 1}
}

// unquant undoes quant.
func unquant(v uint8) float64 { return float64(v) / skySteps }

// skyPalette is every color the sky uses for one theme and state.
type skyPalette struct {
	rows                   []uint32 // gradient, one per frame row
	sun, sunCore, moon     uint32
	moonDark, crater       uint32
	bodyVis, starVis       float64
	star                   uint32
	cloud                  uint32
	cloudVis               float64
	rain, snow, mist, bolt uint32
}

// skyColors is the gradient from the top of the frame to the bottom. It
// stays close to the theme background, tinted by the cool end of the
// theme's ramp by day and its warm end at sunrise and sunset, so text on
// the backdrop keeps its contrast and every theme still reads as itself.
func skyColors(t *ui.Theme, st skyState) (top, bottom canvas.Color) {
	if !st.on {
		return t.BG, t.BG
	}
	light, glow, cast := unquant(st.light), unquant(st.glow), unquant(st.cast)
	cool, warm := t.Heat(0), t.Heat(0.75)
	top = ui.Mix(t.BG, cool, 0.32*light)
	bottom = ui.Mix(t.BG, cool, 0.10+0.40*light)
	top = ui.Mix(top, ui.Mix(t.BG, t.Heat(0.5), 0.10), glow*0.6*(1-cast))
	bottom = ui.Mix(bottom, ui.Mix(t.BG, warm, 0.55), glow*(1-0.6*cast))
	grey := t.Dim
	top = ui.Mix(top, ui.Mix(t.BG, grey, 0.05+0.13*light), cast*0.85)
	bottom = ui.Mix(bottom, ui.Mix(t.BG, grey, 0.08+0.17*light), cast*0.85)
	if st.fog {
		bottom = ui.Mix(bottom, ui.Mix(t.BG, t.FG, 0.10+0.14*light), 0.85)
		top = ui.Mix(top, bottom, 0.35)
	}
	return top, bottom
}

func newSkyPalette(t *ui.Theme, st skyState, h int) *skyPalette {
	top, bottom := skyColors(t, st)
	p := &skyPalette{rows: make([]uint32, h)}
	for y := range h {
		p.rows[y] = rgb(ui.Mix(top, bottom, float64(y)/float64(max(h-1, 1))))
	}
	light, cast := unquant(st.light), unquant(st.cast)
	p.sun = rgb(ui.Mix(t.Heat(0.75), t.FG, 0.15))
	p.sunCore = rgb(ui.Mix(t.Heat(0.7), canvas.RGB(255, 255, 255), 0.45))
	p.moon = rgb(ui.Mix(t.BG, t.FG, 0.78))
	p.moonDark = rgb(ui.Mix(t.BG, t.Faint, 0.6))
	p.crater = rgb(ui.Mix(t.BG, t.FG, 0.58))
	p.bodyVis = 1 - 0.85*cast
	p.star = rgb(t.FG)
	p.starVis = 1
	if st.on {
		p.starVis = (1 - light) * (1 - light) * (1 - cast)
	}
	// Clouds are see-through, pale, greyer and darker the heavier the sky.
	p.cloud = rgb(ui.Mix(ui.Mix(t.Dim, t.FG, 0.4), t.Faint, cast*0.6))
	p.cloudVis = 0.45 + 0.55*light
	p.rain = rgb(ui.Mix(t.Dim, t.Heat(0), 0.4))
	p.snow = rgb(ui.Mix(t.FG, t.BG, 0.12))
	p.mist = rgb(ui.Mix(t.BG, t.FG, 0.45))
	p.bolt = rgb(ui.Mix(t.FG, canvas.RGB(255, 255, 255), 0.6))
	return p
}

// Per-pixel sky bits in Renderer.px: open backdrop (no window), a layer
// pixel drawn last frame, one drawn this frame, and its color below.
const (
	pxOpen  = 1 << 26
	pxSeen  = 1 << 25
	pxDrawn = 1 << 24
	pxColor = 1<<24 - 1
)

// updateSky repaints the sky in the base layer when its state changed
// (the light steps, the sun moves a pixel, the weather turns) and marks the
// tiles that changed for redraw.
func (r *Renderer) updateSky(t *ui.Theme, s Sky) {
	st := skyStateOf(s, r.base.W, r.base.H, r.zenith)
	if st == r.sky {
		return
	}
	old := r.sky
	r.sky = st
	if !sameGradient(old, st) {
		r.skyPal = newSkyPalette(t, st, r.base.H)
		r.paintSky(rect{0, 0, r.base.W, r.base.H}, true)
		return
	}
	r.paintSky(old.bodyRect(), true)
	r.paintSky(st.bodyRect(), true)
}

// paintSky paints the gradient and sun or moon into the open backdrop of
// the base layer within rc, marking changed tiles when track is set.
func (r *Renderer) paintSky(rc rect, track bool) {
	rc = rc.clip(r.base.W, r.base.H)
	if rc.empty() {
		return
	}
	body := r.sky.bodyRect()
	w := r.base.W
	for y := rc.y0; y < rc.y1; y++ {
		row := r.skyPal.rows[y]
		for x := rc.x0; x < rc.x1; x++ {
			i := y*w + x
			if r.px[i]&pxOpen == 0 {
				continue
			}
			v := row
			if x >= body.x0 && x < body.x1 && y >= body.y0 && y < body.y1 {
				v = r.bodyPixel(x, y, row)
			}
			if r.base.Pix[i] != v {
				r.base.Pix[i] = v
				if track {
					r.redraw[(y/Tile)*r.cols+x/Tile] = true
				}
			}
		}
	}
}

// bodyPixel is the sky color v at (x, y) with the sun or moon over it.
func (r *Renderer) bodyPixel(x, y int, v uint32) uint32 {
	st, p := r.sky, r.skyPal
	dx, dy := float64(x-st.bx), float64(y-st.by)
	d := math.Hypot(dx, dy)
	if st.sun {
		switch {
		case d <= bodyR-2.5:
			return lerpRGB(v, p.sunCore, p.bodyVis)
		case d <= bodyR+0.3:
			return lerpRGB(v, p.sun, p.bodyVis)
		case d <= haloR:
			// Glow in hard rings, like a palette-limited sprite.
			k := 1 - (d-bodyR)/(haloR-bodyR)
			return lerpRGB(v, p.sun, math.Ceil(k*k*5)/5*0.5*p.bodyVis)
		}
		return v
	}
	if d > bodyR+0.3 {
		if d <= haloR/2 {
			k := 1 - (d-bodyR)/(haloR/2-bodyR)
			return lerpRGB(v, p.moon, math.Ceil(k*k*4)/4*0.22*p.bodyVis)
		}
		return v
	}
	// Lit part from the phase: the terminator is an ellipse whose width
	// follows cos(phase); waxing lights the right side.
	nx, ny := dx/bodyR, dy/bodyR
	s := math.Sqrt(max(0, 1-ny*ny))
	ph := float64(st.moon) / 32
	k := math.Cos(2 * math.Pi * ph)
	lit := nx > k*s
	if ph > 0.5 {
		lit = nx < -k*s
	}
	c := lerpRGB(v, p.moonDark, 0.7)
	if lit {
		c = p.moon
		if math.Hypot(nx+0.3, ny+0.25) < 0.24 || math.Hypot(nx-0.3, ny-0.3) < 0.18 || math.Hypot(nx-0.05, ny+0.5) < 0.14 {
			c = p.crater
		}
	}
	return lerpRGB(v, c, p.bodyVis)
}

// zenithRow is the middle of the gap between windows in the top third of
// the frame with the most open sky, where the sun passes at noon so that
// it shows. A gap is a run of rows at least half open.
func (r *Renderer) zenithRow() int {
	w, h := r.base.W, r.base.H
	best, bestSum := bodyR+3, 0
	y0, sum := -1, 0
	for y := 0; y <= h/3; y++ {
		n := 0
		if y < h/3 {
			for _, v := range r.px[y*w : (y+1)*w] {
				n += int(v >> 26 & 1)
			}
		}
		switch {
		case n*2 >= w:
			if y0 < 0 {
				y0, sum = y, 0
			}
			sum += n
		case y0 >= 0:
			if sum > bestSum {
				best, bestSum = (y0+y-1)/2, sum
			}
			y0 = -1
		}
	}
	return best
}

// blankTile reports whether a cell leaves the backdrop showing: nothing
// drawn, no fill of its own and no title tab cut into a border.
func blankTile(cell canvas.Cell, t *ui.Theme, onTop bool) bool {
	if cell.BG.Set && cell.BG != t.BG && cell.BG != t.Panel {
		return false
	}
	if onTop && cell.Ch != '─' {
		return false
	}
	return cell.Ch == ' ' || cell.Ch == 0 || frameRune(cell.Ch)
}

// put draws one sky-layer pixel, color v blended k over what is there,
// on open backdrop in blank tiles only, so it never touches a window or
// a glyph.
func (r *Renderer) put(x, y int, v uint32, k float64) {
	if x < 0 || y < 0 || x >= r.frame.W || y >= r.frame.H {
		return
	}
	i := y*r.frame.W + x
	p := r.px[i]
	if p&pxOpen == 0 || !r.blank[(y/Tile)*r.cols+x/Tile] {
		return
	}
	if k < 1 {
		under := r.base.Pix[i]
		if p&pxSeen != 0 {
			under = p & pxColor
		}
		v = lerpRGB(under, v, k)
	}
	if p&pxSeen == 0 {
		r.layNew = append(r.layNew, int32(i))
	}
	if p&pxDrawn == 0 || p&pxColor != v {
		r.frame.Pix[i] = v
		r.markSky(x, y)
	}
	r.px[i] = pxOpen | pxSeen | pxDrawn | v
}

// markSky records a changed sky-layer pixel. Particles are scattered over
// the whole frame, so they get spans of their own: widening each row's
// span to reach them would blit most of the frame every frame.
func (r *Renderer) markSky(x, y int) {
	if n := len(r.skySpans); n > 0 {
		if l := &r.skySpans[n-1]; l.Y == y && l.X1 == x {
			l.X1++
			return
		}
	}
	r.skySpans = append(r.skySpans, Span{Y: y, X0: x, X1: x + 1})
}

// drawSkyLayer draws what moves in the sky over the cached backdrop:
// twinkling stars, drifting clouds, fog, rain, snow and lightning. Each
// frame redraws the whole layer from a.T; pixels that were drawn last
// frame and not this one go back to the base.
func (r *Renderer) drawSkyLayer(a Anim) {
	p, w, h := r.skyPal, r.frame.W, r.frame.H
	wx := weatherFX(a.Sky.Kind)
	if !r.sky.on {
		wx = fx{}
	}
	T := a.T

	if p.starVis > 0.04 {
		for _, st := range r.stars {
			lvl := 2.0
			if T != 0 {
				v := 0.5 + 0.5*math.Sin(T*(0.6+float64(st.h%7)/5)+float64(st.h%628)/100)
				lvl = float64(int(v * (starLevels - 1)))
			}
			r.put(st.x, st.y, p.star, (0.15+0.45*lvl/(starLevels-1))*p.starVis)
		}
	}

	flash := 0.0
	if wx.lightning && T != 0 {
		flash = r.flash(T)
	}
	if wx.clouds > 0 {
		r.drawClouds(wx.clouds, T, flash > 0)
	}
	if wx.fog {
		for k := range h / 10 {
			hv := hash(uint32(k)+101, 31337)
			y := int(hv % uint32(h))
			n := 24 + int(hv>>9%64)
			speed := 2 + float64(hv>>17%5)
			x0 := drift(float64(hv>>3%uint32(w+n)), speed, T, w+n) - n
			for i := range n {
				x := x0 + i
				if (x+y)%2 != 0 {
					continue
				}
				r.put(x, y, p.mist, 0.28*math.Sin(math.Pi*float64(i)/float64(n)))
			}
		}
	}
	if wx.rain {
		n := w * h / 250
		if wx.lightning {
			n = n * 3 / 2
		}
		for k := range n {
			hv := hash(uint32(k)+7, 4243)
			speed := 140 + float64(hv>>8%80)
			y0 := drift(float64(hv>>12%uint32(h+8)), speed, T, h+8) - 8
			x0 := int(hv % uint32(w+h/3))
			for j := range 4 {
				y := y0 + j
				r.put(x0-y/3, y, p.rain, 0.3+0.15*float64(j))
			}
		}
	}
	if wx.snow {
		for k := range w * h / 300 {
			hv := hash(uint32(k)+3, 977)
			speed := 10 + float64(hv>>8%20)
			y := drift(float64(hv>>14%uint32(h+2)), speed, T, h+2) - 1
			x := int(hv%uint32(w)) + int(math.Round(2.5*math.Sin(T*0.9+float64(hv%628)/100)))
			r.put(x, y, p.snow, 0.85)
			if hv%5 == 0 {
				r.put(x-1, y, p.snow, 0.4)
				r.put(x+1, y, p.snow, 0.4)
				r.put(x, y-1, p.snow, 0.4)
				r.put(x, y+1, p.snow, 0.4)
			}
		}
	}

	if flash > 0 {
		for _, i := range r.open {
			r.put(int(i)%w, int(i)/w, p.bolt, flash)
		}
		r.bolt(T, p.bolt)
	}

	for _, i := range r.layOld {
		if v := r.px[i]; v&pxSeen == 0 && v&pxDrawn != 0 {
			r.frame.Pix[i] = r.base.Pix[i]
			r.px[i] = v & pxOpen
			r.markSky(int(i)%w, int(i)/w)
		}
	}
	for _, i := range r.layNew {
		r.px[i] &^= pxSeen
	}
	r.layOld, r.layNew = r.layNew, r.layOld[:0]
}

// drift is where something starting at x0 moving speed px/s is at t,
// wrapped into [0, span).
func drift(x0, speed, t float64, span int) int {
	v, s := x0+speed*t, float64(span)
	return int(v - math.Floor(v/s)*s)
}

// cloud is a pixel-art cloud: 0 clear, 1 lit top, 2 body, 3 shaded base.
type cloud struct {
	w, h int
	px   []uint8
}

var clouds = func() []cloud {
	out := make([]cloud, 6)
	for i := range out {
		out[i] = newCloud(hash(uint32(i)+1, 2718))
	}
	return out
}()

// newCloud draws a puffy cloud from a seed: a few round puffs on a flat base.
func newCloud(seed uint32) cloud {
	w, h := 34+int(seed%5)*10, 11+int(seed>>4%3)*3
	n := 1 + w/h + int(seed>>8%2)
	type puff struct{ x, y, r float64 }
	puffs := make([]puff, n)
	for k := range puffs {
		hv := hash(seed, uint32(k))
		rad := float64(h) * (0.3 + float64(hv%15)/100)
		if k == n/2 {
			rad = float64(h) * 0.5
		}
		x := float64(w) * (float64(k) + 0.5) / float64(n)
		puffs[k] = puff{x, float64(h) - rad, rad}
	}
	in := func(x, y int) bool {
		if x < 0 || y < 0 || x >= w || y >= h {
			return false
		}
		if y >= h*2/3 && x >= w/10 && x < w-w/10 {
			return true
		}
		for _, p := range puffs {
			dx, dy := float64(x)+0.5-p.x, float64(y)+0.5-p.y
			if dx*dx+dy*dy <= p.r*p.r && y < h {
				return true
			}
		}
		return false
	}
	c := cloud{w: w, h: h, px: make([]uint8, w*h)}
	for y := range h {
		for x := range w {
			switch {
			case !in(x, y):
			case !in(x, y-2) || !in(x-1, y-1):
				c.px[y*w+x] = 1
			case y >= h-3 || !in(x+1, y+2):
				c.px[y*w+x] = 3
			default:
				c.px[y*w+x] = 2
			}
		}
	}
	return c
}

// drawClouds drifts cover-many clouds slowly left to right across the
// upper part of the sky.
func (r *Renderer) drawClouds(cover, T float64, flash bool) {
	p, w, h := r.skyPal, r.frame.W, r.frame.H
	col := p.cloud
	if flash {
		col = lerpRGB(col, p.bolt, 0.5)
	}
	alpha := [4]float64{0, 0.55 * p.cloudVis, 0.36 * p.cloudVis, 0.22 * p.cloudVis}
	n := int(math.Ceil(cover * float64(w) / 95))
	for k := range n {
		hv := hash(uint32(k)+11, 1618)
		c := clouds[hv%uint32(len(clouds))]
		y0 := int(hv >> 8 % uint32(max(h*3/5, 1)))
		speed := 1.5 + float64(hv>>16%30)/10
		x0 := drift(float64(hv>>4%uint32(w+c.w)), speed, T, w+c.w) - c.w
		for y := range c.h {
			for x := range c.w {
				if v := c.px[y*c.w+x]; v != 0 {
					r.put(x0+x, y0+y, col, alpha[v])
				}
			}
		}
	}
}

// flash is how bright a storm's lightning lights the sky at T: a double
// flicker on most 7-second slots, else 0.
func (r *Renderer) flash(T float64) float64 {
	const period = 7.0
	n := math.Floor(T / period)
	hv := hash(uint32(int64(n)), 99)
	if hv%3 == 0 {
		return 0
	}
	u := T - n*period - float64(hv>>4%40)/10
	switch {
	case u >= 0 && u < 0.1:
		return 0.45
	case u >= 0.17 && u < 0.27:
		return 0.3
	}
	return 0
}

// bolt draws the lightning bolt of T's slot, jagged with one branch.
func (r *Renderer) bolt(T float64, v uint32) {
	hv := hash(uint32(int64(math.Floor(T/7))), 99)
	w, h := r.frame.W, r.frame.H
	x := int(hv >> 8 % uint32(w))
	end := h * (35 + int(hv>>20%30)) / 100
	branch := end / 3
	bx := 0
	for y := range end {
		j := int(hash(uint32(y/3), hv)%3) - 1
		x += j
		r.put(x, y, v, 1)
		r.put(x+1, y, v, 0.35)
		if y >= branch && y < branch+end/4 {
			if y == branch {
				bx = x
			}
			bx += int(hash(uint32(y/2), hv+1) % 2)
			r.put(bx, y, v, 0.6)
		}
	}
}
