package gfx

import (
	"math"
	"unicode/utf8"

	"orb/internal/canvas"
	"orb/internal/ui"
)

// The dog: a small companion in the header whose behavior is a status
// signal (ui.DogMood). Hand-drawn 20x16 frames facing right, mirrored to
// face left; every pose is a pure function of the mood, its pen and the
// animation clock, so incremental and full renders agree.

const dogW, dogH = 20, 16

type dogFrame int

const (
	dogSit1 dogFrame = iota
	dogSit2
	dogPant1
	dogPant2
	dogBark
	dogNap1
	dogNap2
	dogTrot1
	dogTrot2
	dogSprint1
	dogSprint2
	dogFrames
)

// dogArt: '.' transparent, o outline (theme-tinted), f fur, d dark fur
// (ear, far legs), l cream (muzzle, chest, paws), e eye, n nose, c collar
// (the header accent), g tag, t tongue.
var dogArt = [dogFrames][dogH]string{
	dogSit1: {
		"....................",
		"...........ooooo....",
		"..........offfffo...",
		".........offffffoo..",
		"........odddffeffffo",
		".......oddddffefllln",
		".......oddddfffllllo",
		".......oddddoffoooo.",
		"........oddoccccco..",
		".........oo.offglfo.",
		"....oo....offflllfo.",
		"...ofo...offfflllfo.",
		"...ofo..offfffllfo..",
		"....oofffffffofflfo.",
		".....offffffofoflfo.",
		"......ooooooooooooo.",
	},
	dogSit2: {
		"....................",
		"...........ooooo....",
		"..........offfffo...",
		".........offffffoo..",
		"........odddffeffffo",
		".......oddddffefllln",
		".......oddddfffllllo",
		".......oddddoffoooo.",
		"........oddoccccco..",
		".........oo.offglfo.",
		"..........offflllfo.",
		"..oooo...offfflllfo.",
		".offffooofffffllfo..",
		"..oooooffffffofflfo.",
		".....offffffofoflfo.",
		"......ooooooooooooo.",
	},
	dogPant1: {
		"....................",
		"...........ooooo....",
		"..........offfffo...",
		".........offffffoo..",
		"........odddffeffffo",
		".......oddddffefllln",
		".......oddddfffoooo.",
		".......oddddofftto..",
		"........oddoccctto..",
		".........oo.offgtfo.",
		"....oo....offflllfo.",
		"...ofo...offfflllfo.",
		"...ofo..offfffllfo..",
		"....oofffffffofflfo.",
		".....offffffofoflfo.",
		"......ooooooooooooo.",
	},
	dogPant2: {
		"....................",
		"....................",
		"...........ooooo....",
		"..........offfffo...",
		".........offffffoo..",
		"........odddffeffffo",
		".......oddddffefllln",
		".......oddddfffoooo.",
		".......oddddofftto..",
		"........oddoccctco..",
		"....oo...oo.offglfo.",
		"...ofo...offfflllfo.",
		"...ofo..offfffllfo..",
		"....oofffffffofflfo.",
		".....offffffofoflfo.",
		"......ooooooooooooo.",
	},
	dogBark: {
		"...........ooooo....",
		"..........offfffo...",
		".........offffffoo..",
		"........odddffeffffo",
		".......oddddffefllln",
		".......oddddffooooo.",
		".......oddddofttlo..",
		"........oddoccooo...",
		".........oo.offglfo.",
		"..........offflllfo.",
		"....oo...offfflllfo.",
		"...ofo..offfffllfo..",
		"...ofo.offffflflfo..",
		"....offffffofflflo..",
		".....offfffofoflfo..",
		"......ooooooooooooo.",
	},
	dogNap1: {
		"....................",
		"....................",
		"....................",
		"....................",
		"....................",
		"....................",
		"....................",
		"...........ooooo....",
		"..........offfffo...",
		"....oooo.offffffoo..",
		"..ooffffodddfffffffo",
		".offfffoddddfeefllln",
		"offffffoddddfffllllo",
		"offffffoddddoffoooo.",
		".offlllfffooolllllo.",
		"..oooooooooooooooo..",
	},
	dogNap2: {
		"....................",
		"....................",
		"....................",
		"....................",
		"....................",
		"....................",
		"....................",
		"...........ooooo....",
		"...oooo...offfffo...",
		"..offffffoffffffoo..",
		".offffffodddfffffffo",
		".offfffoddddfeefllln",
		"offffffoddddfffllllo",
		"offffffoddddoffoooo.",
		".offlllfffooolllllo.",
		"..oooooooooooooooo..",
	},
	dogTrot1: {
		"....................",
		"...........ooooo....",
		"..........offfffo...",
		".........offffffoo..",
		"........odddffeffffo",
		".o.....oddddffefllln",
		"ofo....oddddfffllllo",
		"ofo....oddddoffoooo.",
		".ofooooooddoccccco..",
		"..offffffoofffgffo..",
		"..offffffffffffllfo.",
		"..offffffffffffllfo.",
		"...offllllllllfffo..",
		"..oddoffo....oddoffo",
		"..oddoffo....oddoffo",
		"...oo.oo......oo.oo.",
	},
	dogTrot2: {
		"....................",
		"...........ooooo....",
		"..........offfffo...",
		".........offffffoo..",
		"........odddffeffffo",
		".o.....oddddffefllln",
		"ofo....oddddfffllllo",
		"ofo....oddddoffoooo.",
		".ofooooooddoccccco..",
		"..offffffoofffgffo..",
		"..offffffffffffllfo.",
		"..offffffffffffllfo.",
		"...offllllllllfffo..",
		"....offo.....offo...",
		"....offo.....offo...",
		".....oo.......oo....",
	},
	dogSprint1: {
		"....................",
		"...........ooooo....",
		"..........offfffo...",
		"....ooooooffffffoo..",
		"..ooddddddffffeffffo",
		".oddooooffffffefllln",
		".oo....offfffffllllo",
		"ooo.....offffffoooo.",
		"offoooooofffccccco..",
		".oooffffffffffgffo..",
		"..offffffffffffllfo.",
		"..offffffffffffllfo.",
		"...offllllllllfffo..",
		".oddoffo....oddoffo.",
		"oddoffo......oddoffo",
		".oo.oo........oo.oo.",
	},
	dogSprint2: {
		"....................",
		"...........ooooo....",
		"..........offfffo...",
		"....ooooooffffffoo..",
		"..ooddddddffffeffffo",
		".oddooooffffffefllln",
		".oo....offfffffllllo",
		"ooo.....offffffoooo.",
		"offoooooofffccccco..",
		".oooffffffffffgffo..",
		"..offffffffffffllfo.",
		"..offffffffffffllfo.",
		"...offllllllllfffo..",
		".....offo.oddo......",
		"......offooddo......",
		".......oo..oo.......",
	},
}

// dogPalette maps art bytes to pixels; ok marks the opaque ones.
type dogPalette struct {
	v  [128]uint32
	ok [128]bool
}

func newDogPalette(t *ui.Theme, accent canvas.Color) *dogPalette {
	p := &dogPalette{}
	set := func(b byte, c canvas.Color) { p.v[b], p.ok[b] = rgb(c), true }
	set('o', ui.Mix(ui.Mix(t.BG, canvas.RGB(0, 0, 0), 0.5), accent, 0.2))
	set('f', canvas.RGB(0xe8, 0xa2, 0x5a))
	set('d', canvas.RGB(0xa5, 0x64, 0x3a))
	set('l', canvas.RGB(0xff, 0xf0, 0xd6))
	set('e', canvas.RGB(0x1c, 0x10, 0x18))
	set('n', canvas.RGB(0x1c, 0x10, 0x18))
	set('c', ui.Mix(accent, canvas.RGB(255, 255, 255), 0.1))
	set('g', canvas.RGB(0xff, 0xd2, 0x3d))
	set('t', canvas.RGB(0xff, 0x6e, 0x8c))
	return p
}

// dogPose is where the dog is and which frame it shows.
type dogPose struct {
	frame dogFrame
	x, y  int  // top-left of the sprite
	left  bool // facing left (mirrored)
}

// Speeds in logical pixels per second.
const (
	dogTrotSpeed   = 22.0
	dogSprintSpeed = 75.0
)

// dogPoseAt places the dog in pen pr at time T. It stands on the pen's
// bottom edge, one pixel up so its shadow fits. Resting moods sit at the
// right end facing into the screen; trotting and sprinting run laps.
func dogPoseAt(m ui.Mood, pr rect, T float64) dogPose {
	lo, hi := pr.x0+2, pr.x1-dogW-2
	p := dogPose{x: hi, y: pr.y1 - 1 - dogH, left: true}
	step := func(period float64, n int) int { return int(math.Floor(T/period)) % n }
	lap := func(speed float64) {
		span := float64(hi - lo)
		if span <= 0 {
			return
		}
		d := math.Mod(T*speed, 2*span)
		if d < span {
			p.x, p.left = lo+int(d), false
		} else {
			p.x, p.left = hi-int(d-span), true
		}
	}
	switch m {
	case ui.Nap:
		p.frame = dogNap1 + dogFrame(step(1.1, 2)) // breathing
	case ui.Sit:
		p.frame = dogSit1 + dogFrame(step(0.22, 2)) // tail wag
		// Looks around: every 4 s, a 1.3 s glance the other way.
		p.left = math.Mod(T, 4) < 2.7
	case ui.Trot:
		p.frame = dogTrot1 + dogFrame(step(0.16, 2))
		lap(dogTrotSpeed)
	case ui.Sprint:
		p.frame = dogSprint1 + dogFrame(step(0.08, 2))
		lap(dogSprintSpeed)
	case ui.Pant:
		p.frame = dogPant1 + dogFrame(step(0.16, 2))
	case ui.Bark:
		// "Woof woof", then a breath: open mouth twice per 1.2 s.
		u := math.Mod(T, 1.2)
		p.frame = dogSit1
		if u < 0.18 || (u >= 0.32 && u < 0.5) {
			p.frame = dogBark
		}
	}
	// Airborne frames of a gait lift a pixel.
	if p.frame == dogTrot2 || p.frame == dogSprint2 {
		p.y--
	}
	return p
}

// dogMouth is where the mouth is in a right-facing frame (for barks).
const dogMouthX, dogMouthY = 19, 5

// dog draws the companion and its effects: shadow, nap z's, sprint dust,
// bark lines and the speech bubble.
func (r *Renderer) dog(pt canvas.Pet, t *ui.Theme, a Anim) overlay {
	pr := rect{pt.X * Tile, pt.Y * Tile, (pt.X + pt.W) * Tile, (pt.Y + pt.H) * Tile}
	m := ui.Mood(pt.Mood)
	p := dogPoseAt(m, pr, a.T)
	if r.dogPal == nil || r.dogPal.theme != t || r.dogPal.accent != pt.Accent {
		r.dogPal = &dogPalCache{t, pt.Accent, newDogPalette(t, pt.Accent)}
	}
	pal := r.dogPal.p
	ground := pr.y1 - 1

	// Everything drawn stays inside bb: the sprite with room for its
	// effects, widened for the bubble.
	bb := rect{p.x - 8, pr.y0, p.x + dogW + 8, pr.y1}
	var bubble rect
	var lines [2]string
	if m == ui.Bark {
		lines = [2]string{"Woof!", pt.Say}
		bw := max(7*utf8.RuneCountInString(lines[0]), 6*utf8.RuneCountInString(lines[1])) + 7
		bubble = rect{p.x - 7 - bw, pr.y0 + 2, p.x - 7, ground}
		if !p.left { // mouth on the right: bubble there
			bubble = rect{p.x + dogW + 7, pr.y0 + 2, p.x + dogW + 7 + bw, ground}
		}
		bb = rect{min(bb.x0, bubble.x0-1), bb.y0, max(bb.x1, bubble.x1+4), bb.y1}
	}
	bb = rect{max(bb.x0, pr.x0-Tile), bb.y0, min(bb.x1, pr.x1+Tile), bb.y1}

	return overlay{bb, func(f *Frame) {
		blend := func(x, y int, v uint32, k float64) {
			if x >= bb.x0 && x < bb.x1 && y >= bb.y0 && y < bb.y1 && x >= 0 && y >= 0 && x < f.W && y < f.H {
				i := y*f.W + x
				f.Pix[i] = lerpRGB(f.Pix[i], v, k)
			}
		}
		put := func(x, y int, v uint32) { blend(x, y, v, 1) }

		// Shadow: a soft line under the paws.
		x0, x1 := p.x+2, p.x+dogW-1
		if m == ui.Nap {
			x0 = p.x
		}
		for x := x0; x < x1; x++ {
			k := 0.45
			if x == x0 || x == x1-1 {
				k = 0.2
			}
			blend(x, ground, 0, k)
		}

		art := dogArt[p.frame]
		for sy, row := range art {
			for sx := range dogW {
				b := row[sx]
				if p.left {
					b = row[dogW-1-sx]
				}
				if b < 128 && pal.ok[b] {
					put(p.x+sx, p.y+sy, pal.v[b])
				}
			}
		}

		fg := rgb(t.FG)
		switch m {
		case ui.Nap:
			// Three z's drift up from the head, staggered.
			hx := p.x + 13
			if p.left {
				hx = p.x + 3
			}
			for k := range 3 {
				const life = 2.4
				u := math.Mod(a.T+float64(k)*life/3, life) / life
				fade := min(u/0.15, 1, (1-u)/0.3)
				zx := hx + int(u*6)
				if p.left {
					zx = hx - int(u*6) - 3
				}
				zy := p.y + 6 - int(u*12)
				zz := [4]string{"####", "..#.", ".#..", "####"}
				if k%2 == 1 {
					zz = [4]string{"###.", ".#..", "###.", "...."}
				}
				for y, row := range zz {
					for x := range len(row) {
						if row[x] == '#' {
							blend(zx+x, zy+y, fg, fade)
						}
					}
				}
			}
		case ui.Sprint:
			// Dust kicked up behind.
			for k := range 3 {
				u := math.Mod(a.T*3+float64(k)/3, 1)
				dx := 2 + int(u*7)
				x := p.x - dx
				if p.left {
					x = p.x + dogW - 1 + dx
				}
				y := ground - 1 - int(u*3)
				c := rgb(ui.Mix(t.Dim, t.FG, 0.3))
				blend(x, y, c, 0.7*(1-u))
				blend(x+1, y, c, 0.5*(1-u))
				blend(x, y-1, c, 0.4*(1-u))
			}
		case ui.Pant:
			// Sweat drops flicking off the back of the head.
			drop := rgb(canvas.RGB(0x9b, 0xdc, 0xff))
			for k := range 2 {
				u := math.Mod(a.T/0.9+float64(k)*0.5, 1)
				dx := int(u * 7)
				hx := p.x + 7 - dx // behind the head (art faces right)
				if p.left {
					hx = p.x + dogW - 8 + dx
				}
				hy := p.y + 1 - int(8*u*(1-u)*2.2) + int(u*4)
				fade := min(1, (1-u)/0.35)
				blend(hx, hy, drop, fade)
				blend(hx, hy+1, drop, fade)
				blend(hx+1, hy+1, drop, fade)
				blend(hx-1, hy+1, drop, 0.5*fade)
			}
		case ui.Bark:
			hot := rgb(t.Heat(1))
			if p.frame == dogBark {
				// Sound lines fanning out from the mouth.
				mx, dir := p.x+dogMouthX+2, 1
				if p.left {
					mx, dir = p.x+dogW-1-dogMouthX-2, -1
				}
				for _, d := range [3][2]int{{1, -2}, {2, 0}, {1, 2}} {
					for s := range 2 {
						put(mx+dir*(d[0]+s), p.y+dogMouthY+d[1]*(1+s)/2, hot)
					}
				}
			}
			drawBubble(f, bubble, !p.left, lines, t, put)
		}
	}}
}

type dogPalCache struct {
	theme  *ui.Theme
	accent canvas.Color
	p      *dogPalette
}

// drawBubble draws an RPG speech bubble: cream fill, dark rim, a tail
// toward the dog, the first line in the hot color and the rest dark.
func drawBubble(f *Frame, b rect, tailLeft bool, lines [2]string, t *ui.Theme, put func(x, y int, v uint32)) {
	if b.empty() {
		return
	}
	rim := rgb(ui.Mix(t.BG, canvas.RGB(0, 0, 0), 0.4))
	fill := rgb(canvas.RGB(0xff, 0xf6, 0xe4))
	for y := b.y0; y < b.y1; y++ {
		for x := b.x0; x < b.x1; x++ {
			switch {
			case inRound(x, y, b.x0+1, b.y0+1, b.x1-1, b.y1-1, 2):
				put(x, y, fill)
			case inRound(x, y, b.x0, b.y0, b.x1, b.y1, 3):
				put(x, y, rim)
			}
		}
	}
	// Tail: a small wedge at mouth height pointing at the dog.
	ty := b.y0 + 9
	for i := range 3 {
		for j := -2 + i; j <= 2-i; j++ {
			x := b.x1 - 1 + i
			if tailLeft {
				x = b.x0 - i
			}
			v := fill
			if j == -2+i || j == 2-i {
				v = rim
			}
			put(x, ty+j, v)
		}
	}
	ink := []uint32{rgb(alarmInk(t)), rgb(ui.Mix(t.BG, canvas.RGB(0, 0, 0), 0.2))}
	for li, s := range lines {
		y := b.y0 + 3 + li*9
		x := b.x0 + 4
		for _, ch := range s {
			g, ok := font[ch]
			if !ok {
				g = font['?']
			}
			if li == 0 { // bold
				for gy := range g {
					g[gy] |= g[gy] >> 1
				}
			}
			for gy, row := range g {
				for gx := range Tile {
					if row&(0x80>>gx) != 0 {
						put(x+gx-1, y+gy, ink[li])
					}
				}
			}
			x += 6
			if li == 0 {
				x++
			}
		}
	}
}

// alarmInk is the hottest color of the theme's ramp that still reads on
// the bubble's cream fill (some themes end the ramp near white), darkened
// as a last resort.
func alarmInk(t *ui.Theme) canvas.Color {
	lum := func(c canvas.Color) float64 { return 0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B) }
	for v := 1.0; v >= 0.6; v -= 0.05 {
		if c := t.Heat(v); lum(c) < 150 {
			return c
		}
	}
	c := t.Heat(1)
	for range 8 {
		if lum(c) < 150 {
			break
		}
		c = ui.Mix(c, canvas.RGB(0, 0, 0), 0.18)
	}
	return c
}
