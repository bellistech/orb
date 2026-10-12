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

// Rasterize draws canvas c into f, which must be c.W*Tile x c.H*Tile.
func Rasterize(f *Frame, c *canvas.Canvas, t *ui.Theme, a Anim) {
	if f == nil || c == nil || t == nil || f.W != c.W*Tile || f.H != c.H*Tile {
		return
	}
	drawBackdrop(f, t, a)
	tops := make(map[int]int) // cell index on a window's top edge -> window
	for i, w := range c.Windows {
		drawWindow(f, w, t)
		for x := w.X + 1; x < w.X+w.W-1; x++ {
			tops[w.Y*c.W+x] = i
		}
	}
	for y := range c.H {
		for x := range c.W {
			cell := c.CellAt(x, y)
			wi, onTop := tops[y*c.W+x]
			if onTop && cell.Ch != '─' {
				// Cut a tab out of the border behind title text.
				top, _, _ := t.Window(c.Windows[wi].Accent)
				f.fill(x*Tile, y*Tile, Tile, Tile, rgb(top))
			}
			drawCell(f, x, y, cell, t, shimmer(c, wi, onTop, x, a))
		}
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

// drawBackdrop paints the space between windows: the theme background with
// a sparse field of slowly twinkling stars.
func drawBackdrop(f *Frame, t *ui.Theme, a Anim) {
	bg := rgb(t.BG)
	for i := range f.Pix {
		f.Pix[i] = bg
	}
	for y := 0; y < f.H; y++ {
		for x := 0; x < f.W; x++ {
			h := hash(uint32(x), uint32(y))
			if h%997 != 0 {
				continue
			}
			tw := 0.35
			if a.T != 0 {
				tw = 0.2 + 0.4*(0.5+0.5*math.Sin(a.T*(0.6+float64(h%7)/5)+float64(h%628)/100))
			}
			f.set(x, y, rgb(ui.Mix(t.BG, t.FG, tw)))
		}
	}
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
	var sh glyph
	for y := 1; y < Tile; y++ {
		sh[y] = g[y-1] >> 1
	}
	blit(f, x0, y0, sh, rgb(shadow))
	blit(f, x0, y0, g, rgb(fg))
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
