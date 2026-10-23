package gfx

import (
	"math"

	"orb/internal/canvas"
	"orb/internal/ui"
)

// rect is a pixel rectangle [x0,x1) x [y0,y1).
type rect struct{ x0, y0, x1, y1 int }

func (r rect) clip(w, h int) rect {
	return rect{max(r.x0, 0), max(r.y0, 0), min(r.x1, w), min(r.y1, h)}
}

func (r rect) empty() bool { return r.x0 >= r.x1 || r.y0 >= r.y1 }

// overlay is pixel art drawn over the composed frame, ignoring the tile
// grid. Tiles under an overlay this frame or the last are redrawn first,
// so overlays never leave trails.
type overlay struct {
	r    rect
	draw func(f *Frame)
}

func lerpRGB(a, b uint32, t float64) uint32 {
	t = max(0, min(1, t))
	ch := func(s uint) uint32 {
		x, y := float64(a>>s&0xFF), float64(b>>s&0xFF)
		return uint32(x+(y-x)*t+0.5) << s
	}
	return ch(16) | ch(8) | ch(0)
}

// overlays lists this frame's effects: the orb emblem, the dog and its
// ball, and edge sparkles.
func (r *Renderer) overlays(c *canvas.Canvas, t *ui.Theme, a Anim) []overlay {
	var out []overlay
	for _, o := range c.Orbs {
		out = append(out, r.orb(o, t, a))
	}
	for _, p := range c.Pets {
		out = append(out, r.dog(p, t, a)...)
	}
	return append(out, sparkles(c, t, a)...)
}

// orbPalette caches the sphere's colors: mist level x distance band.
type orbPalette struct {
	theme  *ui.Theme
	accent canvas.Color
	cols   [5][8]uint32
	shine  uint32
	rim    uint32
}

func (r *Renderer) palette(t *ui.Theme, accent canvas.Color) *orbPalette {
	if r.orbPal != nil && r.orbPal.theme == t && r.orbPal.accent == accent {
		return r.orbPal
	}
	p := &orbPalette{theme: t, accent: accent}
	for m := range 5 {
		for d := range 8 {
			p.cols[m][d] = rgb(ui.OrbColor(t, accent, float64(m)/4, float64(d)/7))
		}
	}
	_, _, edge := t.Window(accent)
	p.shine = rgb(canvas.RGB(255, 255, 255))
	p.rim = rgb(ui.Mix(edge, accent, 0.3))
	r.orbPal = p
	return p
}

// orb draws the emblem filling the cell rect.
func (r *Renderer) orb(o canvas.Window, t *ui.Theme, a Anim) overlay {
	pr := rect{o.X * Tile, o.Y * Tile, (o.X + o.W) * Tile, (o.Y + o.H) * Tile}
	pal := r.palette(t, o.Accent)
	return overlay{pr, func(f *Frame) { drawOrb(f, pr, pal, rgb(o.Accent), a) }}
}

// drawOrb draws a glass sphere filling pixel rect pr: swirling quantized
// mist (palette cycling), a darker limb, a highlight, a glassy rim and a
// soft pulsing halo blended over what is behind it.
func drawOrb(f *Frame, pr rect, pal *orbPalette, halo uint32, a Anim) {
	w, h := pr.x1-pr.x0, pr.y1-pr.y0
	R := float64(min(w, h))/2 - 2
	if R < 4 {
		return
	}
	cx, cy := float64(pr.x0)+float64(w)/2-0.5, float64(pr.y0)+float64(h)/2-0.5
	pulse := 0.35 + 0.15*math.Sin(a.T*1.7)
	cr := pr.clip(f.W, f.H)
	for y := cr.y0; y < cr.y1; y++ {
		for x := cr.x0; x < cr.x1; x++ {
			dx, dy := (float64(x)-cx)/R, (float64(y)-cy)/R
			d := math.Hypot(dx, dy)
			i := y*f.W + x
			switch {
			case d <= 1:
				m := int(ui.OrbMist(dx, dy, a.T) * 4)
				v := pal.cols[max(0, min(m, 4))][min(int(d*7), 7)]
				if math.Hypot(dx+0.38, dy+0.4) < 0.2 {
					v = lerpRGB(v, pal.shine, 0.85)
				} else if d > 0.9 {
					v = lerpRGB(v, pal.rim, 0.4)
				}
				f.Pix[i] = v
			case d <= 1+3/R:
				g := (1 - (d-1)*R/3) * pulse
				f.Pix[i] = lerpRGB(f.Pix[i], halo, g)
			}
		}
	}
}

// sparkles returns the glints blooming along window edges at a.T: a few
// slots on staggered periods, each briefly growing a four-pointed star
// somewhere on a window's bevel, then fading.
func sparkles(c *canvas.Canvas, t *ui.Theme, a Anim) []overlay {
	if a.T == 0 || len(c.Windows) == 0 {
		return nil
	}
	const slots, life = 6, 0.9
	var out []overlay
	for k := range slots {
		period := 5 + float64(k)*1.7
		ph := a.T + float64(k)*2.3
		n := math.Floor(ph / period)
		u := ph - n*period
		if u >= life {
			continue
		}
		hv := hash(uint32(k+1)*7919, uint32(int64(n)))
		w := c.Windows[hv%uint32(len(c.Windows))]
		x0, y0 := w.X*Tile+3, w.Y*Tile+3
		x1, y1 := (w.X+w.W)*Tile-4, (w.Y+w.H)*Tile-4
		if x1 <= x0 || y1 <= y0 {
			continue
		}
		per := 2 * ((x1 - x0) + (y1 - y0))
		p := int(hv>>8) % per
		var sx, sy int
		switch {
		case p < x1-x0:
			sx, sy = x0+p, y0
		case p < (x1-x0)+(y1-y0):
			sx, sy = x1, y0+p-(x1-x0)
		case p < 2*(x1-x0)+(y1-y0):
			sx, sy = x1-(p-(x1-x0)-(y1-y0)), y1
		default:
			sx, sy = x0, y1-(p-2*(x1-x0)-(y1-y0))
		}
		grow := math.Sin(math.Pi * u / life)
		_, _, edge := t.Window(w.Accent)
		core := rgb(ui.Mix(edge, canvas.RGB(255, 255, 255), 0.7))
		out = append(out, overlay{rect{sx - 3, sy - 3, sx + 4, sy + 4}, func(f *Frame) {
			glint(f, sx, sy, 3, grow, core)
		}})
	}
	return out
}

// glint blends a four-pointed star into f at (sx, sy): arms up to size
// pixels long, grown by grow (0..1), fading toward their tips.
func glint(f *Frame, sx, sy, size int, grow float64, core uint32) {
	set := func(x, y int, k float64) {
		if x >= 0 && y >= 0 && x < f.W && y < f.H {
			i := y*f.W + x
			f.Pix[i] = lerpRGB(f.Pix[i], core, k)
		}
	}
	set(sx, sy, grow)
	arm := int(math.Round(float64(size) * grow))
	for d := 1; d <= arm; d++ {
		k := grow * (1 - float64(d)/float64(size+1))
		set(sx+d, sy, k)
		set(sx-d, sy, k)
		set(sx, sy+d, k)
		set(sx, sy-d, k)
	}
}
