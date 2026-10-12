// Package gfx draws the dashboard as pixels: 8x8 tiles, a hand-drawn font,
// and windows in the style of 16-bit console RPG menus. Output goes to the
// Linux framebuffer or a PNG.
package gfx

import (
	"math"

	"nocterm/internal/canvas"
	"nocterm/internal/ui"
)

// Frame is a logical (unscaled) image, one 0xRRGGBB value per pixel.
type Frame struct {
	W, H int
	Pix  []uint32
}

// NewFrame returns a w x h frame. Sizes are clamped to be non-negative.
func NewFrame(w, h int) *Frame {
	w, h = max(w, 0), max(h, 0)
	return &Frame{W: w, H: h, Pix: make([]uint32, w*h)}
}

func rgb(c canvas.Color) uint32 { return uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B) }

func (f *Frame) set(x, y int, v uint32) {
	if x >= 0 && y >= 0 && x < f.W && y < f.H {
		f.Pix[y*f.W+x] = v
	}
}

func (f *Frame) fill(x, y, w, h int, v uint32) {
	for yy := max(y, 0); yy < min(y+h, f.H); yy++ {
		for xx := max(x, 0); xx < min(x+w, f.W); xx++ {
			f.Pix[yy*f.W+xx] = v
		}
	}
}

// Anim carries time for animated effects. The zero value is a still frame.
type Anim struct {
	T float64 // seconds
}

// Rasterize draws canvas c into f, which must be c.W*Tile x c.H*Tile. It is
// the stateless form of Renderer.Draw, for one-off frames such as PNGs.
func Rasterize(f *Frame, c *canvas.Canvas, t *ui.Theme, a Anim) {
	if f == nil || c == nil || t == nil || f.W != c.W*Tile || f.H != c.H*Tile {
		return
	}
	r := NewRenderer()
	r.Draw(c, t, a)
	copy(f.Pix, r.Frame().Pix)
}

// Span is a changed run of logical pixels [X0, X1) on row Y.
type Span struct{ Y, X0, X1 int }

// Renderer keeps the previous frame so each Draw only redraws what changed.
// The backdrop and windows are a cached base layer; tiles are restored from
// it and redrawn only when their cell, or its shimmer, changed.
type Renderer struct {
	base, frame *Frame
	key         string // theme + window layout the base was built for
	cells       []canvas.Cell
	glow        []float64
	stars       []star
	dirty       []Span // per row, X0 >= X1 means clean
}

type star struct {
	x, y int
	h    uint32
}

// NewRenderer returns an empty renderer; the first Draw is a full frame.
func NewRenderer() *Renderer { return &Renderer{} }

// Frame is the current composed frame. Callers must not modify it.
func (r *Renderer) Frame() *Frame { return r.frame }

// Draw updates the frame to show c and returns the rows that changed.
func (r *Renderer) Draw(c *canvas.Canvas, t *ui.Theme, a Anim) []Span {
	if c == nil || t == nil {
		return nil
	}
	if key := baseKey(c, t); r.frame == nil || r.frame.W != c.W*Tile || r.frame.H != c.H*Tile || key != r.key {
		r.rebuild(c, t, key)
	}
	for i := range r.dirty {
		r.dirty[i] = Span{Y: i, X0: r.frame.W, X1: 0}
	}

	tops := make(map[int]int, len(c.Windows)*16)
	for i, w := range c.Windows {
		for x := w.X + 1; x < w.X+w.W-1; x++ {
			tops[w.Y*c.W+x] = i
		}
	}
	for y := range c.H {
		for x := range c.W {
			i := y*c.W + x
			cell := c.CellAt(x, y)
			wi, onTop := tops[i]
			glow := shimmer(c, wi, onTop, x, a)
			if r.cells != nil && cell == r.cells[i] && glow == r.glow[i] {
				continue
			}
			r.restoreTile(x, y)
			if onTop && cell.Ch != '─' {
				// Cut a tab out of the border behind title text.
				top, _, _ := t.Window(c.Windows[wi].Accent)
				r.frame.fill(x*Tile, y*Tile, Tile, Tile, rgb(top))
			}
			drawCell(r.frame, x, y, cell, t, glow)
			r.mark(x*Tile, y*Tile, Tile, Tile)
			if r.cells == nil {
				continue
			}
			r.cells[i], r.glow[i] = cell, glow
		}
	}
	if r.cells == nil {
		r.cells = make([]canvas.Cell, c.W*c.H)
		r.glow = make([]float64, c.W*c.H)
		for y := range c.H {
			for x := range c.W {
				r.cells[y*c.W+x] = c.CellAt(x, y)
			}
		}
	}
	r.drawStars(t, a)

	out := r.dirty[:0:0]
	for _, d := range r.dirty {
		if d.X0 < d.X1 {
			out = append(out, d)
		}
	}
	return out
}

// baseKey identifies what the base layer depends on.
func baseKey(c *canvas.Canvas, t *ui.Theme) string {
	b := make([]byte, 0, 64+len(c.Windows)*20)
	b = append(b, t.Name...)
	for _, w := range c.Windows {
		b = append(b, byte(w.X), byte(w.X>>8), byte(w.Y), byte(w.W), byte(w.W>>8), byte(w.H), w.Accent.R, w.Accent.G, w.Accent.B)
	}
	return string(b)
}

func (r *Renderer) rebuild(c *canvas.Canvas, t *ui.Theme, key string) {
	w, h := c.W*Tile, c.H*Tile
	r.key = key
	r.base, r.frame = NewFrame(w, h), NewFrame(w, h)
	bg := rgb(t.BG)
	for i := range r.base.Pix {
		r.base.Pix[i] = bg
	}
	for _, win := range c.Windows {
		drawWindow(r.base, win, t)
	}
	copy(r.frame.Pix, r.base.Pix)
	// Stars only where the backdrop shows through.
	r.stars = r.stars[:0]
	for y := range h {
		for x := range w {
			if hv := hash(uint32(x), uint32(y)); hv%997 == 0 && r.base.Pix[y*w+x] == bg {
				r.stars = append(r.stars, star{x, y, hv})
			}
		}
	}
	r.cells, r.glow = nil, nil
	r.dirty = make([]Span, h)
	for i := range r.dirty {
		r.dirty[i] = Span{Y: i, X0: 0, X1: w}
	}
}

func (r *Renderer) restoreTile(cx, cy int) {
	w := r.frame.W
	for y := cy * Tile; y < (cy+1)*Tile; y++ {
		o := y*w + cx*Tile
		copy(r.frame.Pix[o:o+Tile], r.base.Pix[o:o+Tile])
	}
}

func (r *Renderer) mark(x, y, w, h int) {
	for yy := y; yy < y+h && yy < len(r.dirty); yy++ {
		d := &r.dirty[yy]
		d.X0, d.X1 = min(d.X0, x), max(d.X1, x+w)
	}
}

// starLevels quantizes twinkle brightness so most frames change no stars.
const starLevels = 6

// drawStars twinkles the backdrop stars, touching only those whose
// brightness step changed.
func (r *Renderer) drawStars(t *ui.Theme, a Anim) {
	for _, s := range r.stars {
		lvl := uint8(2)
		if a.T != 0 {
			v := 0.5 + 0.5*math.Sin(a.T*(0.6+float64(s.h%7)/5)+float64(s.h%628)/100)
			lvl = uint8(v * (starLevels - 1))
		}
		px := &r.frame.Pix[s.y*r.frame.W+s.x]
		want := rgb(ui.Mix(t.BG, t.FG, 0.15+0.45*float64(lvl)/(starLevels-1)))
		if *px == want {
			continue
		}
		*px = want
		r.mark(s.x, s.y, 1, 1)
	}
}

// shimmer is how much a title cell is brightened by the light sweeping
// across each window's title every few seconds, staggered per window.
func shimmer(c *canvas.Canvas, wi int, onTop bool, x int, a Anim) float64 {
	if !onTop || a.T == 0 {
		return 0
	}
	const period = 7.0
	w := c.Windows[wi]
	phase := math.Mod(a.T+float64(wi)*1.3, period) / 1.2 // sweep takes 1.2s
	if phase > 1 {
		return 0
	}
	pos := float64(w.X) + phase*float64(min(w.W, 30))
	return max(0, 1-math.Abs(float64(x)-pos)/2.5) * 0.7
}

func hash(x, y uint32) uint32 {
	h := x*374761393 + y*668265263
	h = (h ^ (h >> 13)) * 1274126177
	return h ^ (h >> 16)
}

// drawWindow draws a rounded RPG-menu window over the panel's cells: dark
// outline, light bevel, dark inner line, vertical gradient fill. The border
// runs through the middle of the edge tiles, where box lines would be.
func drawWindow(f *Frame, w canvas.Window, t *ui.Theme) {
	x0, y0 := w.X*Tile+2, w.Y*Tile+2
	x1, y1 := (w.X+w.W)*Tile-2, (w.Y+w.H)*Tile-2
	if x1-x0 < 10 || y1-y0 < 10 {
		return
	}
	top, bot, edge := t.Window(w.Accent)
	outline := rgb(ui.Mix(t.BG, canvas.RGB(0, 0, 0), 0.6))
	inner := rgb(ui.Mix(bot, canvas.RGB(0, 0, 0), 0.5))
	edgeLit := rgb(edge)
	edgeDim := rgb(ui.Mix(edge, w.Accent, 0.45))
	h := float64(y1 - y0)
	for y := y0; y < y1; y++ {
		fillRow := rgb(ui.Mix(top, bot, float64(y-y0)/h))
		for x := x0; x < x1; x++ {
			var v uint32
			switch {
			case inRound(x, y, x0+4, y0+4, x1-4, y1-4, 2):
				v = fillRow
			case inRound(x, y, x0+3, y0+3, x1-3, y1-3, 2):
				v = inner
			case inRound(x, y, x0+1, y0+1, x1-1, y1-1, 3):
				// Lit from the top-left like a bevel.
				if x-x0 < y-y0 && x1-x > y1-y || y-y0 <= 2 {
					v = edgeLit
				} else {
					v = edgeDim
				}
			case inRound(x, y, x0, y0, x1, y1, 4):
				v = outline
			default:
				continue
			}
			f.set(x, y, v)
		}
	}
}

// inRound reports whether (x, y) lies in the rectangle [x0,x1) x [y0,y1)
// with corners rounded to radius r.
func inRound(x, y, x0, y0, x1, y1, r int) bool {
	if x < x0 || y < y0 || x >= x1 || y >= y1 {
		return false
	}
	cx, cy := x, y
	switch {
	case x < x0+r:
		cx = x0 + r
	case x >= x1-r:
		cx = x1 - r - 1
	}
	switch {
	case y < y0+r:
		cy = y0 + r
	case y >= y1-r:
		cy = y1 - r - 1
	}
	dx, dy := float64(x-cx), float64(y-cy)
	return dx*dx+dy*dy <= float64(r*r)+0.5
}

// drawCell paints one tile: its background (unless it is the plain panel or
// backdrop color, which the windows already provide), then its glyph.
func drawCell(f *Frame, cx, cy int, cell canvas.Cell, t *ui.Theme, glow float64) {
	x0, y0 := cx*Tile, cy*Tile
	if cell.BG.Set && cell.BG != t.BG && cell.BG != t.Panel {
		f.fill(x0, y0, Tile, Tile, rgb(cell.BG))
	}
	if cell.Ch == ' ' || cell.Ch == 0 || frameRune(cell.Ch) {
		return
	}
	fg := cell.FG
	if !fg.Set {
		fg = t.FG
	}
	if glow > 0 {
		fg = ui.Mix(fg, canvas.RGB(255, 255, 255), glow)
	}
	if g, ok := shape(cell.Ch); ok {
		blit(f, x0, y0, g, rgb(fg))
		return
	}
	g, ok := font[cell.Ch]
	if !ok {
		g = font['?']
	}
	if cell.Bold {
		for y := range g {
			g[y] |= g[y] >> 1
		}
	}
	shadow := ui.Mix(fg, canvas.RGB(0, 0, 0), 0.75)
	if cell.Quad != 0 {
		g = quarter(g, cell.Quad)
		sh := quarter(shift(font[cell.Ch], cell.Bold), cell.Quad)
		blit(f, x0, y0, sh, rgb(shadow))
		blit(f, x0, y0, g, rgb(fg))
		return
	}
	blit(f, x0, y0, shift(g, false), rgb(shadow))
	blit(f, x0, y0, g, rgb(fg))
}

// shift returns g's drop shadow: the glyph moved one pixel down and right
// (bolded first when bold, matching the glyph it shadows).
func shift(g glyph, bold bool) glyph {
	if bold {
		for y := range g {
			g[y] |= g[y] >> 1
		}
	}
	var sh glyph
	for y := 1; y < Tile; y++ {
		sh[y] = g[y-1] >> 1
	}
	return sh
}

// quarter returns quadrant q (1..4) of g scaled up 2x.
func quarter(g glyph, q uint8) glyph {
	var out glyph
	if q < 1 || q > 4 {
		return out
	}
	ox, oy := int(q-1)%2*Tile/2, int(q-1)/2*Tile/2 // source offset in pixels
	for y := range Tile {
		src := g[oy+y/2]
		for x := range Tile {
			if src&(0x80>>(ox+x/2)) != 0 {
				out[y] |= 0x80 >> x
			}
		}
	}
	return out
}

func blit(f *Frame, x0, y0 int, g glyph, v uint32) {
	for y, row := range g {
		for x := range Tile {
			if row&(0x80>>x) != 0 {
				f.set(x0+x, y0+y, v)
			}
		}
	}
}
