package gfx

import (
	"math"
	"sync"
	"time"

	"orb/internal/canvas"
	"orb/internal/feeds"
	"orb/internal/ui"
)

// Henry Mode's meadow: the sky above (the backdrop's own sky, sun, moon,
// stars and weather), hills, a field with flowers, a stream and a tree
// painted into the base layer, and the creatures as overlays. Henry
// chases butterflies by day and naps at night; a turtle walks the bank;
// birds cross the day sky; an owl and fireflies come out at night. Every
// pose is a pure function of the scene rectangle and the animation clock,
// so incremental and full renders agree.

// HenrySky is the sky Henry Mode shows: the clock's, or a time of day
// pinned by the arrow keys, on today's date.
func HenrySky(h ui.HenryTime, now time.Time, fv feeds.View) Sky {
	s := NewSky(now, fv)
	if h == ui.HenryClock {
		return s
	}
	rise, set := sunTimes(now)
	hours := map[ui.HenryTime]float64{ui.HenryDawn: rise + 0.15, ui.HenryDay: (rise + set) / 2, ui.HenryDusk: set - 0.15, ui.HenryNight: 0.5}[h]
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	s.Now = day.Add(time.Duration(hours * float64(time.Hour)))
	return s
}

// meadow is the scene's layout in logical pixels.
type meadow struct {
	pr                     rect
	w, h                   int
	horizon, ground        int // the far edge of the field; Henry's paws
	streamMid, streamAmp   float64
	streamHalf             int
	treeX, treeBase        int
	trunkH, trunkW, canopy int
	s                      int // sprite scale: Henry and his friends grow with the scene
}

func meadowOf(pr rect) meadow {
	w, h := pr.x1-pr.x0, pr.y1-pr.y0
	m := meadow{pr: pr, w: w, h: h}
	m.horizon = pr.y0 + h*50/100
	m.ground = pr.y0 + h*66/100
	m.streamMid = float64(pr.y0) + float64(h)*0.82
	m.streamAmp = float64(h) * 0.03
	m.streamHalf = max(4, h*3/100)
	m.treeX = pr.x0 + w*86/100
	m.treeBase = pr.y0 + h*61/100
	short := min(w, h*2) // a tall scene must not grow a tree wider than itself
	m.trunkH = max(16, short*10/100)
	m.trunkW = max(4, short*2/100)
	m.canopy = max(14, short*9/100) // big enough to hold the owl
	m.s = max(2, min(4, h/130))
	return m
}

// skyline is the top of the hills at x.
func (m meadow) skyline(x int) int {
	u := float64(x - m.pr.x0)
	hh := float64(m.h)*0.05*(0.5+0.5*math.Sin(u*2*math.Pi/(float64(m.w)*0.9)+1)) + float64(m.h)*0.022*(0.5+0.5*math.Sin(u*0.031+2))
	return m.horizon - int(hh) - 2
}

// streamY is the stream's center line at x.
func (m meadow) streamY(x int) int {
	u := float64(x-m.pr.x0) / float64(max(m.w, 1))
	return int(m.streamMid + m.streamAmp*math.Sin(u*2*math.Pi*1.2+0.7))
}

// meadowLight is how lit the meadow is: the sky's daylight, dimmed under
// cloud, and its dawn or dusk glow.
func meadowLight(s Sky) (light, glow float64) {
	if s.Now.IsZero() {
		return 0, 0 // the plain starfield: night
	}
	light, glow = daylight(s.Now)
	cast := weatherFX(s.Kind).overcast
	return light * (1 - 0.45*cast), glow * (1 - cast)
}

// meadowKey changes when the painted meadow must be repainted.
func meadowKey(s Sky) string {
	l, g := meadowLight(s)
	return string([]byte{'m', quant(l), quant(g)})
}

// tint darkens toward a night blue as the light goes and warms at dawn
// and dusk.
func tint(v uint32, light, glow float64) uint32 {
	v = lerpRGB(v, 0xff9a5a, glow*0.18)
	return lerpRGB(v, 0x0a1226, (1-light)*0.78)
}

// paintMeadow paints the ground of the scene into f: everything below the
// hills. What is above stays untouched, so it becomes open sky.
func paintMeadow(f *Frame, pr rect, s Sky) {
	m := meadowOf(pr)
	light, glow := meadowLight(s)
	cr := pr.clip(f.W, f.H)
	for x := cr.x0; x < cr.x1; x++ {
		top, sy := m.skyline(x), m.streamY(x)
		for y := max(top, cr.y0); y < cr.y1; y++ {
			var v uint32
			hv := hash(uint32(x), uint32(y))
			switch d := y - sy; {
			case y < m.horizon: // distant hills, hazy toward their foot
				k := float64(y-top) / float64(max(m.horizon-top, 1))
				v = lerpRGB(0x5f8f78, 0x7fae7a, k)
				if hv%41 == 0 {
					v = lerpRGB(v, 0x4c7a63, 0.6)
				}
			case d > -m.streamHalf && d < m.streamHalf: // water
				k := math.Abs(float64(d)) / float64(m.streamHalf)
				v = lerpRGB(0x2f6fb0, 0x6fb6de, k*k)
				if hv%29 == 0 {
					v = lerpRGB(v, 0xbfe6f5, 0.35)
				}
			case d > -m.streamHalf-3 && d < m.streamHalf+3: // banks
				v = 0x8a7a4e
				if hv%5 == 0 {
					v = 0x6f6440
				}
			default: // the field, richer toward the front
				k := float64(y-m.horizon) / float64(max(cr.y1-m.horizon, 1))
				v = lerpRGB(0x8fc46a, 0x3f8f3a, k)
				switch {
				case hv%13 == 0:
					v = lerpRGB(v, 0x2f6e2c, 0.5)
				case hv%17 == 0:
					v = lerpRGB(v, 0xb6e08a, 0.4)
				}
			}
			f.Pix[y*f.W+x] = tint(v, light, glow)
		}
	}
	// Flowers on a coarse grid: a dot with petals, more toward the front.
	petals := []uint32{0xff8fb1, 0xffd84a, 0xf6f3ea, 0xb48cff, 0xff7a5a}
	for gy := m.horizon + 6; gy < cr.y1-2; gy += 5 {
		for gx := cr.x0 + 2; gx < cr.x1-2; gx += 7 {
			hv := hash(uint32(gx)*7+3, uint32(gy)*13+1)
			density := 3 + 8*(gy-m.horizon)/max(cr.y1-m.horizon, 1)
			if int(hv%40) >= density {
				continue
			}
			x, y := gx+int(hv>>8%5), gy+int(hv>>12%3)
			if sy := m.streamY(x); y > sy-m.streamHalf-5 && y < sy+m.streamHalf+5 {
				continue
			}
			c := tint(petals[hv>>16%uint32(len(petals))], light, glow)
			mid := tint(0xffe066, light, glow)
			for _, p := range [][2]int{{0, -1}, {-1, 0}, {1, 0}, {0, 1}} {
				f.set(x+p[0], y+p[1], c)
			}
			f.set(x, y, mid)
			f.set(x, y+2, tint(0x2f6e2c, light, glow))
		}
	}
	paintTree(f, m, light, glow)
}

// paintTree draws the tree the owl sits in: a trunk, a branch reaching
// left and a canopy of overlapping blobs lit from the upper left.
func paintTree(f *Frame, m meadow, light, glow float64) {
	bark, barkDark := tint(0x6b4a2b, light, glow), tint(0x4a3220, light, glow)
	tx, top := m.treeX, m.treeBase-m.trunkH
	for y := top; y < m.treeBase; y++ {
		for x := tx - m.trunkW/2; x < tx+(m.trunkW+1)/2; x++ {
			v := bark
			if x >= tx+m.trunkW/4 || hash(uint32(x), uint32(y))%7 == 0 {
				v = barkDark
			}
			f.set(x, y, v)
		}
	}
	// The owl's branch, at the canopy's foot.
	by := top + m.canopy/3
	for x := tx - m.canopy - 6; x < tx; x++ {
		f.set(x, by, bark)
		f.set(x, by+1, barkDark)
	}
	blobs := [][3]float64{{0, -0.6, 1}, {-0.75, -0.1, 0.75}, {0.75, -0.15, 0.78}, {-0.3, 0.3, 0.7}, {0.35, 0.3, 0.72}, {0, -1.25, 0.6}}
	R := float64(m.canopy)
	for _, b := range blobs {
		cx, cy, r := float64(tx)+b[0]*R, float64(top)+b[1]*R, b[2]*R
		for y := int(cy - r); y <= int(cy+r); y++ {
			for x := int(cx - r); x <= int(cx+r); x++ {
				dx, dy := (float64(x)-cx)/r, (float64(y)-cy)/r
				if dx*dx+dy*dy > 1 {
					continue
				}
				k := (dx + dy + 1.4) / 2.8 // 0 lit, 1 shade
				v := lerpRGB(0x6fbf4f, 0x2c6b2e, k)
				if hash(uint32(x), uint32(y))%9 == 0 {
					v = lerpRGB(v, 0x245a26, 0.6)
				}
				f.set(x, y, tint(v, light, glow))
			}
		}
	}
}

// owlPerch is where the owl's feet rest.
func (m meadow) owlPerch() (x, y int) {
	return m.treeX - m.canopy*2/3, m.treeBase - m.trunkH + m.canopy/3
}

// putter returns a pixel writer clipped to bb and the frame, blending by k.
func putter(f *Frame, bb rect) func(x, y int, v uint32, k float64) {
	bb = bb.clip(f.W, f.H)
	return func(x, y int, v uint32, k float64) {
		if x < bb.x0 || x >= bb.x1 || y < bb.y0 || y >= bb.y1 {
			return
		}
		i := y*f.W + x
		if k >= 1 {
			f.Pix[i] = v
		} else {
			f.Pix[i] = lerpRGB(f.Pix[i], v, k)
		}
	}
}

// drawSprite draws art at (x, y) scaled by s, mirrored when flip, with
// colors from pal ('.' and unknown bytes transparent).
func drawSprite(art []string, x, y, s int, flip bool, pal map[byte]uint32, put func(x, y int, v uint32, k float64)) {
	for row, line := range art {
		n := len(line)
		for col := range n {
			v, ok := pal[line[col]]
			if !ok {
				continue
			}
			c := col
			if flip {
				c = n - 1 - col
			}
			for dy := range s {
				for dx := range s {
					put(x+c*s+dx, y+row*s+dy, v, 1)
				}
			}
		}
	}
}

// meadowOverlays lists the scene's creatures for this frame, back to
// front.
func (r *Renderer) meadowOverlays(sc canvas.Window, t *ui.Theme, a Anim) []overlay {
	pr := rect{sc.X * Tile, sc.Y * Tile, (sc.X + sc.W) * Tile, (sc.Y + sc.H) * Tile}
	m := meadowOf(pr)
	if m.w < 80 || m.h < 60 {
		return nil
	}
	light, _ := meadowLight(a.Sky)
	night := light < 0.45
	var out []overlay
	out = append(out, r.meadowClouds(m, a.T, light, a.Sky)...)
	if night {
		out = append(out, owl(m, a.T, light))
	} else {
		out = append(out, birds(m, a.T)...)
	}
	out = append(out, glints(m, a.T, night)...)
	if !night {
		out = append(out, butterflies(m, a.T, light)...)
	}
	out = append(out, r.henry(m, t, a.T, night))
	if !night {
		out = append(out, teaser(m, a.T, light))
	}
	out = append(out, turtle(m, a.T, night, light))
	if night {
		out = append(out, fireflies(m, a.T)...)
	}
	return out
}

// Henry, at twice the header's size.

const (
	henrySeg   = 20.0 // one chase: sit, run, leap, sniff, bow, pant
	henryTrot  = 20.0 // logical pixels per second per unit of scale
	henrySpeed = 55.0
)

// henryLane is where Henry may run, left to right.
func (m meadow) henryLane() (lo, hi int) {
	lo, hi = m.pr.x0+m.w*4/100, m.treeX-m.canopy-m.henryW()
	return lo, max(hi, lo+1)
}

// Henry is drawn from double-resolution art (henryArt), so he stands
// twice the header dog's size at the scene's sprite scale.
func (m meadow) henryW() int { return 2 * dogW * m.s }
func (m meadow) henryH() int { return 2 * dogH * m.s }

// henrySpot is where chase k starts (and chase k-1 ends).
func (m meadow) henrySpot(k int) int {
	lo, hi := m.henryLane()
	return lo + int(hash(uint32(k), 77)%uint32(max(hi-lo, 1)))
}

// henryChase is chase k's timing: start and end spots, when the run
// starts and how long it takes, and whether he sprints.
func (m meadow) henryChase(k int) (a, b int, t0, run float64, sprint bool) {
	a, b = m.henrySpot(k), m.henrySpot(k+1)
	d := math.Abs(float64(b - a))
	sp := 2 * henryTrot * float64(m.s)
	if d > float64(160*m.s) {
		sp, sprint = 2*henrySpeed*float64(m.s), true
	}
	return a, b, 1.5, d / sp, sprint
}

type henryPose struct {
	frame dogFrame
	x     int // left edge
	lift  int // pixels off the ground (a leap)
	left  bool
}

func henryPoseAt(m meadow, T float64, night bool) henryPose {
	if night {
		x := max(m.henrySpot(0), m.treeX-m.canopy-m.henryW()-20)
		f := dogNap1
		if int(T/1.6)%2 == 1 {
			f = dogNap2
		}
		return henryPose{frame: f, x: x}
	}
	k := int(math.Floor(T / henrySeg))
	u := T - float64(k)*henrySeg
	a, b, t0, run, sprint := m.henryChase(k)
	left := b < a
	arr := t0 + run
	alt := func(hz float64, f1, f2 dogFrame) dogFrame {
		if int(u*hz)%2 == 0 {
			return f1
		}
		return f2
	}
	switch {
	case u < t0:
		return henryPose{alt(2, dogSit1, dogSit2), a, 0, left}
	case u < arr:
		f := alt(6, dogTrot1, dogTrot2)
		if sprint {
			f = alt(10, dogSprint1, dogSprint2)
		}
		return henryPose{f, lerpPx(a, b, (u-t0)/max(run, 1e-6)), 0, left}
	case u < arr+0.6:
		return henryPose{dogLeap, b, int(float64(18*m.s) * math.Sin(math.Pi*(u-arr)/0.6)), left}
	case u < arr+1.6:
		return henryPose{dogNose, b, 0, left}
	case u < arr+3.0:
		return henryPose{alt(3, dogBow1, dogBow2), b, 0, left}
	}
	return henryPose{alt(2.5, dogPant1, dogPant2), b, 0, left}
}

func (r *Renderer) henry(m meadow, t *ui.Theme, T float64, night bool) overlay {
	p := henryPoseAt(m, T, night)
	sc := m.s
	w, h := m.henryW(), m.henryH()
	top := m.ground - h - p.lift
	bb := rect{p.x - 6, top - 40, p.x + w + 28, m.ground + 4}
	if r.dogPal == nil || r.dogPal.theme != t || r.dogPal.accent != t.Host {
		r.dogPal = &dogPalCache{t, t.Host, newDogPalette(t, t.Host)}
	}
	pal := r.dogPal.p
	return overlay{bb, func(f *Frame) {
		put := putter(f, bb)
		// A soft shadow, smaller when he is in the air.
		sw := w - 20 - p.lift
		for x := p.x + (w-sw)/2; x < p.x+(w+sw)/2; x++ {
			put(x, m.ground, 0x000000, 0.3)
			put(x, m.ground+1, 0x000000, 0.2)
			put(x, m.ground+2, 0x000000, 0.1)
		}
		drawArt(henryArt(p.frame), 0, 0, p.left, pal, func(x, y int, v uint32) {
			for dy := range sc {
				for dx := range sc {
					put(p.x+x*sc+dx, top+y*sc+dy, v, 1)
				}
			}
		})
		if night { // z's drifting up from a sleeping dog
			for i := range 3 {
				u := math.Mod(T/3+float64(i)/3, 1)
				zx := p.x + w - 16 + int(u*24)
				zy := top + 8 - int(u*36)
				k := math.Sin(math.Pi * u)
				for row, line := range []string{"####", "..#.", ".#..", "####"} {
					for col := range line {
						if line[col] == '#' {
							for d := range 4 {
								put(zx+col*2+d%2, zy+row*2+d/2, 0xe8ecff, k)
							}
						}
					}
				}
			}
		}
	}}
}

// Butterflies, by day.

var butterflyArt = [2][]string{
	{"ww.ww", "wpkpw", ".wkw."},
	{".w.w.", ".wkw.", "..k.."},
}

var butterflyWings = [][2]uint32{{0xff9a1f, 0x1a1a1a}, {0x6ec6ff, 0x2b5fd9}, {0xffe35a, 0xc9a227}, {0xf6f3ea, 0xb8b4ac}, {0xff8fd0, 0xc2418f}}

func butterflyAt(x, y, s int, T float64, i int, light float64) overlay {
	bb := rect{x, y, x + 5*s, y + 3*s}
	frame := int(T*7+float64(i)*0.37) % 2
	c := butterflyWings[i%len(butterflyWings)]
	pal := map[byte]uint32{'w': tint(c[0], light, 0), 'p': tint(c[1], light, 0), 'k': tint(0x1a1410, light, 0)}
	return overlay{bb, func(f *Frame) {
		drawSprite(butterflyArt[frame], x, y, s, false, pal, putter(f, bb))
	}}
}

func butterflies(m meadow, T float64, light float64) []overlay {
	var out []overlay
	for i := range 4 {
		fi := float64(i)
		cx := float64(m.pr.x0) + float64(m.w)*(0.12+0.2*fi) + float64(m.w)*0.07*math.Sin(T*0.13+fi*1.7)
		cy := float64(m.horizon) + float64(m.ground-m.horizon)*0.4 + float64(m.h)*0.05*math.Sin(T*0.21*(1+0.1*fi)+fi*2)
		cx += 4 * math.Sin(T*2.3+fi)
		cy += 3 * math.Sin(T*3.1+fi*0.7)
		out = append(out, butterflyAt(int(cx), int(cy), m.s, T, i+1, light))
	}
	return out
}

// teaser is the butterfly Henry chases: it hovers where his run ends,
// dips to his nose as he leaps, flees up and drifts to the next spot.
func teaser(m meadow, T float64, light float64) overlay {
	k := int(math.Floor(T / henrySeg))
	u := T - float64(k)*henrySeg
	_, b, t0, run, _ := m.henryChase(k)
	_, b2, _, _, _ := m.henryChase(k + 1)
	arr := t0 + run
	head := float64(m.ground - m.henryH() - 8)
	hover := func(x int, u float64) (float64, float64) {
		return float64(x+m.henryW()/2) + float64(12*m.s)*math.Sin(u*1.9), head - float64(20*m.s) + float64(6*m.s)*math.Sin(u*2.6)
	}
	var x, y float64
	hx, hy := hover(b, u)
	fx, fy := float64(b+m.henryW()/2+30*m.s), float64(m.horizon)-float64(m.h)*0.05
	nx, ny := hover(b2, 0)
	switch {
	case u < arr:
		x, y = hx, hy
	case u < arr+0.4: // down to his nose
		v := (u - arr) / 0.4
		x, y = hx, hy+(head-hy)*v
	case u < arr+3:
		v := smoothstep(0, 1, (u-arr-0.4)/2.6)
		x, y = hx+(fx-hx)*v, head+(fy-head)*v
	default:
		v := smoothstep(0, 1, (u-arr-3)/max(henrySeg-arr-3, 1))
		x, y = fx+(nx-fx)*v, fy+(ny-fy)*v
	}
	x += 3 * math.Sin(T*4.1)
	y += 2 * math.Sin(T*5.3)
	return butterflyAt(int(x), int(y), m.s, T, 0, light)
}

// The turtle on the near bank.

var turtleArt = [3][]string{
	{"....oooooo......", "...oSsSSsSo.....", "..oSsSSsSSSo.kk.", ".oSSsSSsSsSSokke", ".orrrrrrrrrrrkk.", "..kk..kk..kk...."},
	{"....oooooo......", "...oSsSSsSo.....", "..oSsSSsSSSo.kk.", ".oSSsSSsSsSSokke", ".orrrrrrrrrrrkk.", "...kk..kk..kk..."},
	{"................", "....oooooo......", "...oSsSSsSo.....", "..oSsSSsSSSo....", ".oSSsSSsSsSSo...", ".orrrrrrrrrrro.."},
}

func turtle(m meadow, T float64, night bool, light float64) overlay {
	const period = 300.0
	s := m.s
	lo, hi := m.pr.x0+m.w*8/100, m.pr.x0+m.w*60/100
	u := math.Mod(T/period, 1)
	tri, left := u*2, false
	if u >= 0.5 {
		tri, left = 2-u*2, true
	}
	x := lo + int(float64(hi-lo)*tri)
	frame := int(T*2) % 2
	if night {
		x, frame, left = lo+(hi-lo)*3/10, 2, false
	}
	w, h := 48*s, 18*s // turtleArt, three times over
	feet := m.streamY(x+w/2) - m.streamHalf - 1
	y := feet - h
	if frame < 2 && int(T*1.3)%5 == 0 {
		y -= s // a head bob now and then
	}
	bb := rect{x - 2, y - 2, x + w + 2, feet + 3}
	pal := map[byte]uint32{'o': tint(0x223a20, light, 0), 'S': tint(0x3e6b3a, light, 0), 's': tint(0x7da35a, light, 0),
		'r': tint(0x6b4a2b, light, 0), 'k': tint(0x9bbf6a, light, 0), 'e': 0x111111,
		'H': tint(0xa6d07a, light, 0), 'D': tint(0x2c4f2a, light, 0), 'R': tint(0x8a6440, light, 0), 'K': tint(0x7a9a52, light, 0)}
	return overlay{bb, func(f *Frame) {
		put := putter(f, bb)
		for xx := x + 2; xx < x+w-2; xx++ {
			put(xx, feet+1, 0x000000, 0.25)
		}
		drawSprite(turtleBig(frame), x, y, s, left, pal, put)
	}}
}

// Birds crossing the day sky, far off.

var birdArt = [2][]string{
	{"##...##", ".##.##.", "...#..."},
	{".......", "###.###", "...#..."},
}

func birds(m meadow, T float64) []overlay {
	var out []overlay
	pal := map[byte]uint32{'#': 0x2b2b35}
	s := max(1, m.s-1)
	for i := range 4 {
		fi := float64(i)
		span := float64(m.w + 60)
		x := m.pr.x0 - 30 + int(math.Mod(T*(18+5*fi)+fi*span/4, span))
		y := m.pr.y0 + m.h*(9+6*i)/100 + int(3*math.Sin(T*0.9+fi))
		frame := 0
		if math.Sin(T*9+fi*2) < 0 {
			frame = 1
		}
		bb := rect{x, y, x + 7*s, y + 3*s}
		out = append(out, overlay{bb, func(f *Frame) { drawSprite(birdArt[frame], x, y, s, false, pal, putter(f, bb)) }})
	}
	return out
}

// The owl on its branch at night: it blinks and looks about.

var owlArt = []string{
	".o.....o.",
	".oo...oo.",
	"obbbbbbbo",
	"obffbffbo",
	"ofLRfLRfo",
	"obffnffbo",
	"oblllllbo",
	"oblblblbo",
	"obllllllo",
	".obbbbbo.",
	"..t...t..",
}

func owl(m meadow, T float64, light float64) overlay {
	s := m.s
	px, py := m.owlPerch()
	w, h := 27*s, 33*s // owlArt, three times over
	x, y := px-w/2, py-h+s
	blink := math.Mod(T, 5.3) < 0.18
	look := math.Mod(T/7, 1) < 0.5
	L, R := byte('e'), byte('k')
	if look {
		L, R = 'k', 'e'
	}
	lit := light*0.6 + 0.4
	pal := map[byte]uint32{'o': 0x2a1d14, 'b': tint(0x7a5233, lit, 0), 'l': tint(0xc9a27a, lit, 0),
		'f': tint(0xe3cfa8, lit, 0), 'e': 0xffcf3a, 'k': 0x111111, 'n': 0xd18a2a, 't': 0xffb347,
		'B': tint(0x9a6c45, lit, 0), 'd': tint(0x5a3a22, lit, 0), 'm': tint(0xa88660, lit, 0), 'F': tint(0xf4e6c6, lit, 0)}
	art := make([]string, len(owlArt))
	for i, row := range owlArt {
		b := []byte(row)
		for j, c := range b {
			switch {
			case blink && (c == 'L' || c == 'R'):
				b[j] = 'b'
			case c == 'L':
				b[j] = L
			case c == 'R':
				b[j] = R
			}
		}
		art[i] = string(b)
	}
	art = owlDetail(scale3x(art))
	bb := rect{x, y, x + w, y + h}
	return overlay{bb, func(f *Frame) { drawSprite(art, x, y, s, false, pal, putter(f, bb)) }}
}

// Fireflies over the field at night.

func fireflies(m meadow, T float64) []overlay {
	var out []overlay
	for i := range 14 {
		fi := float64(i)
		hv := hash(uint32(i), 901)
		x := m.pr.x0 + int(float64(m.w)*float64(hv%1000)/1000) + int(25*math.Sin(T*0.3*float64(i%3+1)+fi))
		y := m.horizon + m.h*5/100 + int(float64(m.h)*0.28*float64(hv>>10%1000)/1000) + int(12*math.Sin(T*0.47+fi*1.3))
		b := math.Sin(T*2*math.Pi/2.7 + fi*1.9)
		if b <= 0.05 {
			continue
		}
		b *= b
		bb := rect{x - 1, y - 1, x + 2, y + 2}
		out = append(out, overlay{bb, func(f *Frame) {
			put := putter(f, bb)
			put(x, y, 0xfdffa0, b)
			for _, p := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
				put(x+p[0], y+p[1], 0xd8ff6a, b*0.45)
			}
		}})
	}
	return out
}

// Glints drifting down the stream.

func glints(m meadow, T float64, night bool) []overlay {
	var out []overlay
	c, peak := uint32(0xf4fbff), 0.7
	if night {
		c, peak = 0xc8d4ff, 0.45
	}
	for i := range 16 {
		fi := float64(i)
		x := m.pr.x0 + int(math.Mod(T*14+fi*float64(m.w)/16+7*math.Sin(fi), float64(m.w)))
		off := int(hash(uint32(i), 55)%uint32(max(2*m.streamHalf-2, 1))) - m.streamHalf + 1
		y := m.streamY(x) + off
		k := (math.Sin(T*3+fi*2.1) + 1) / 2 * peak
		if k < 0.08 {
			continue
		}
		bb := rect{x, y, x + 3, y + 1}
		out = append(out, overlay{bb, func(f *Frame) {
			put := putter(f, bb)
			put(x, y, c, k)
			put(x+1, y, c, k)
			put(x+2, y, c, k*0.5)
		}})
	}
	return out
}

// The high-detail art, built once: Henry's frames doubled by Scale2x with
// light and shade on his coat, the turtle tripled by Scale3x with a lit
// shell.
var (
	bigOnce    sync.Once
	henryBig   [dogFrames][]string
	turtleBig3 [len(turtleArt)][]string
)

func buildBig() {
	for f := range dogFrames {
		a := scale2x(dogArt[f][:])
		a = shade(a, "w", 'W', 'v', 19) // the white coat: lit back, shaded belly, a little fur
		a = shade(a, "k", 'K', 'k', 0)  // a sheen on the black ears and patches
		a = shade(a, "t", 'T', 't', 0)  // and on the tan points
		henryBig[f] = a
	}
	for i := range turtleArt {
		a := scale3x(turtleArt[i])
		a = shade(a, "Ss", 'H', 'D', 11) // a lit shell with dark scute lines
		a = shade(a, "r", 'R', 'r', 0)
		a = shade(a, "k", 'K', 'k', 0)
		turtleBig3[i] = a
	}
}

func henryArt(f dogFrame) []string {
	bigOnce.Do(buildBig)
	return henryBig[f]
}

func turtleBig(i int) []string {
	bigOnce.Do(buildBig)
	return turtleBig3[i]
}

// owlDetail adds feathers to the tripled owl: lit crown and wings,
// streaks on the chest, a pale rim to the face.
func owlDetail(a []string) []string {
	a = shade(a, "b", 'B', 'd', 7)
	a = shade(a, "l", 'l', 'm', 5)
	return shade(a, "f", 'F', 'f', 0)
}

// Fair-weather clouds drifting across Henry Mode's sky, each a new random
// shape every time it comes round: a row of round puffs, biggest in the
// middle, on a flat base, lit on top and grey underneath. Dim and blue at
// night, warm at dawn and dusk. A shape is worked out once per lap and
// cached; a frame only blends its pixels at the new place.

// cloudPx is one pixel of a cloud shape: where, and which shade (0 body,
// 1 lit top, 2 grey underside; +4 on the soft edge).
type cloudPx struct {
	dx, dy int16
	kind   uint8
}

type cloudShape struct {
	w, h int
	px   []cloudPx
}

func newCloudShape(seed uint32, s int) *cloudShape {
	cw := (36 + int(seed%40)) * s
	ch := cw * 45 / 100
	base := float64(ch) * 0.85
	n := 3 + int(seed>>8%4)
	type puff struct{ cx, cy, r float64 }
	puffs := make([]puff, n)
	for j := range n {
		mid := 1 - math.Abs(float64(j)-float64(n-1)/2)/(float64(n)/2) // 1 in the middle
		r := float64(ch) * (0.22 + 0.25*mid + 0.08*float64(hash(seed, uint32(j))%4)/3)
		cx := float64(cw) * (0.18 + 0.64*float64(j)/float64(max(n-1, 1)))
		puffs[j] = puff{cx, base - r*0.55, r}
	}
	sh := &cloudShape{w: cw, h: ch + 1}
	for y := 0; y <= int(base); y++ {
		for x := range cw {
			best, top := 2.0, 0.0
			for _, p := range puffs {
				dx, dy := float64(x)-p.cx, float64(y)-p.cy
				if d := math.Hypot(dx, dy) / p.r; d < best {
					best, top = d, -dy/p.r
				}
			}
			if best > 1 {
				continue
			}
			var k uint8
			switch {
			case float64(y) > base-float64(ch)*0.18:
				k = 2
			case top > 0.45:
				k = 1
			}
			if best > 0.9 {
				k += 4
			}
			sh.px = append(sh.px, cloudPx{int16(x), int16(y), k})
		}
	}
	return sh
}

// cloudShapeFor returns the cached shape, building it on first use. The
// cache holds the clouds of the last few laps only.
func (r *Renderer) cloudShapeFor(seed uint32, s int) *cloudShape {
	key := [2]uint32{seed, uint32(s)}
	if sh, ok := r.cloudShapes[key]; ok {
		return sh
	}
	if r.cloudShapes == nil || len(r.cloudShapes) >= 16 {
		r.cloudShapes = map[[2]uint32]*cloudShape{}
	}
	sh := newCloudShape(seed, s)
	r.cloudShapes[key] = sh
	return sh
}

func (r *Renderer) meadowClouds(m meadow, T float64, light float64, sky Sky) []overlay {
	_, glow := meadowLight(sky)
	lit := lerpRGB(0x5a6480, 0xffffff, light)
	lit = lerpRGB(lit, 0xffc49a, glow*0.5)
	cols := [3]uint32{lerpRGB(lit, 0x8a94ad, 0.15), lit, lerpRGB(lit, 0x7d88a6, 0.45)}
	alpha := 0.6 + 0.35*light
	var out []overlay
	for i := range 5 {
		fi := float64(i)
		speed := float64(m.s) * (2.5 + 1.5*float64(hash(uint32(i), 3)%4))
		span := float64(m.w) + float64(140*m.s)
		pos := T*speed + fi*span/5
		lap := uint32(pos / span)
		sh := r.cloudShapeFor(hash(uint32(i)*977+13, lap), m.s)
		x := m.pr.x0 - 70*m.s + int(math.Mod(pos, span))
		y := m.pr.y0 + m.h*(3+int(hash(uint32(i), lap+7)%22))/100
		bb := rect{x, y, x + sh.w, y + sh.h}
		out = append(out, overlay{bb, func(f *Frame) {
			put := putter(f, bb)
			for _, p := range sh.px {
				k := alpha
				if p.kind >= 4 {
					k *= 0.6 // a soft edge
				}
				put(x+int(p.dx), y+int(p.dy), cols[p.kind&3], k)
			}
		}})
	}
	return out
}
