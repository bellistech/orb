// Package canvas is a grid of styled cells that panels draw into and that
// serializes to a truecolor ANSI string.
package canvas

import (
	"strconv"
	"strings"
)

// Color is a 24-bit color. The zero value means "unset" (terminal default).
type Color struct {
	R, G, B uint8
	Set     bool
}

// RGB returns a set color.
func RGB(r, g, b uint8) Color { return Color{R: r, G: g, B: b, Set: true} }

// Hex parses "#rrggbb". Invalid input yields an unset color.
func Hex(s string) Color {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return Color{}
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return Color{}
	}
	return RGB(uint8(v>>16), uint8(v>>8), uint8(v))
}

// Cell is one terminal cell.
type Cell struct {
	Ch   rune
	FG   Color
	BG   Color
	Bold bool
}

// Canvas is a W x H grid of cells.
type Canvas struct {
	W, H  int
	cells []Cell
}

// New returns a canvas filled with spaces on bg.
func New(w, h int, bg Color) *Canvas {
	w, h = max(w, 0), max(h, 0)
	c := &Canvas{W: w, H: h, cells: make([]Cell, w*h)}
	for i := range c.cells {
		c.cells[i] = Cell{Ch: ' ', BG: bg}
	}
	return c
}

// Region is a clipped rectangle of a canvas. Coordinates passed to its
// methods are relative to the region; anything outside is dropped.
type Region struct {
	c          *Canvas
	X, Y, W, H int
}

// Full returns a region covering the whole canvas.
func (c *Canvas) Full() Region { return Region{c: c, W: c.W, H: c.H} }

// Sub returns a child region, clipped to r.
func (r Region) Sub(x, y, w, h int) Region {
	x, y = max(x, 0), max(y, 0)
	w = max(min(w, r.W-x), 0)
	h = max(min(h, r.H-y), 0)
	return Region{c: r.c, X: r.X + x, Y: r.Y + y, W: w, H: h}
}

// Empty reports whether nothing can be drawn into r.
func (r Region) Empty() bool { return r.W <= 0 || r.H <= 0 }

func (r Region) at(x, y int) *Cell {
	if x < 0 || y < 0 || x >= r.W || y >= r.H {
		return nil
	}
	return &r.c.cells[(r.Y+y)*r.c.W+r.X+x]
}

// Set draws ch with fg at (x, y), keeping the existing background.
func (r Region) Set(x, y int, ch rune, fg Color) {
	if cell := r.at(x, y); cell != nil {
		cell.Ch, cell.FG, cell.Bold = ch, fg, false
	}
}

// SetBold is Set with bold.
func (r Region) SetBold(x, y int, ch rune, fg Color) {
	if cell := r.at(x, y); cell != nil {
		cell.Ch, cell.FG, cell.Bold = ch, fg, true
	}
}

// SetBG paints the background of (x, y).
func (r Region) SetBG(x, y int, bg Color) {
	if cell := r.at(x, y); cell != nil {
		cell.BG = bg
	}
}

// Text writes s starting at (x, y) and returns the column after it.
// One rune is one cell; callers stick to narrow runes.
func (r Region) Text(x, y int, s string, fg Color) int {
	for _, ch := range s {
		r.Set(x, y, ch, fg)
		x++
	}
	return x
}

// Bold is Text in bold.
func (r Region) Bold(x, y int, s string, fg Color) int {
	for _, ch := range s {
		r.SetBold(x, y, ch, fg)
		x++
	}
	return x
}

// TextRight writes s so that it ends at column x (exclusive) and returns its
// start column.
func (r Region) TextRight(x, y int, s string, fg Color) int {
	start := x - len([]rune(s))
	r.Text(start, y, s, fg)
	return start
}

// String serializes the canvas, emitting SGR only when style changes.
func (c *Canvas) String() string {
	var b strings.Builder
	b.Grow(c.W * c.H * 4)
	for y := 0; y < c.H; y++ {
		var cur Cell
		first := true
		for x := 0; x < c.W; x++ {
			cell := c.cells[y*c.W+x]
			if first || cell.FG != cur.FG || cell.BG != cur.BG || cell.Bold != cur.Bold {
				writeSGR(&b, cell)
				cur, first = cell, false
			}
			b.WriteRune(cell.Ch)
		}
		b.WriteString("\x1b[0m")
		if y < c.H-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func writeSGR(b *strings.Builder, cell Cell) {
	b.WriteString("\x1b[0")
	if cell.Bold {
		b.WriteString(";1")
	}
	if cell.FG.Set {
		b.WriteString(";38;2;")
		writeRGB(b, cell.FG)
	}
	if cell.BG.Set {
		b.WriteString(";48;2;")
		writeRGB(b, cell.BG)
	}
	b.WriteByte('m')
}

func writeRGB(b *strings.Builder, c Color) {
	b.WriteString(strconv.Itoa(int(c.R)))
	b.WriteByte(';')
	b.WriteString(strconv.Itoa(int(c.G)))
	b.WriteByte(';')
	b.WriteString(strconv.Itoa(int(c.B)))
}

// Plain returns the canvas text without styling (for tests and debugging).
func (c *Canvas) Plain() string {
	var b strings.Builder
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			b.WriteRune(c.cells[y*c.W+x].Ch)
		}
		if y < c.H-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// CellAt returns the cell at absolute (x, y); out-of-range yields a zero cell.
func (c *Canvas) CellAt(x, y int) Cell {
	if x < 0 || y < 0 || x >= c.W || y >= c.H {
		return Cell{}
	}
	return c.cells[y*c.W+x]
}

// Set2 is Set returning the next column, for chaining with Text.
func (r Region) Set2(x, y int, ch rune, fg Color) int {
	r.Set(x, y, ch, fg)
	return x + 1
}
