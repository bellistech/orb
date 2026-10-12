package ui

import "nocterm/internal/canvas"

// glyphs is a 3x5 pixel font for the clock. Each string is one pixel row.
var glyphs = map[rune][5]string{
	'0': {"###", "#.#", "#.#", "#.#", "###"},
	'1': {".#.", "##.", ".#.", ".#.", "###"},
	'2': {"###", "..#", "###", "#..", "###"},
	'3': {"###", "..#", ".##", "..#", "###"},
	'4': {"#.#", "#.#", "###", "..#", "..#"},
	'5': {"###", "#..", "###", "..#", "###"},
	'6': {"###", "#..", "###", "#.#", "###"},
	'7': {"###", "..#", ".#.", ".#.", ".#."},
	'8': {"###", "#.#", "###", "#.#", "###"},
	'9': {"###", "#.#", "###", "..#", "###"},
	':': {".", "#", ".", "#", "."},
	' ': {".", ".", ".", ".", "."},
}

// BigWidth is the cell width of s drawn by Big.
func BigWidth(s string, square bool) int {
	px := pixelW(square)
	w := 0
	for i, ch := range s {
		if i > 0 {
			w++
		}
		w += len(glyphs[ch][0]) * px
	}
	return w
}

// pixelW is how many cells wide one font pixel is: two on a terminal's tall
// cells, one on square tiles, so the digits keep the same shape.
func pixelW(square bool) int {
	if square {
		return 1
	}
	return 2
}

// Big draws s three cells tall using half blocks. color picks a color per
// column offset so the text can carry a gradient.
func Big(r canvas.Region, x, y int, s string, color func(col int) canvas.Color) int {
	pw := pixelW(r.Square())
	col := 0
	for i, ch := range s {
		if i > 0 {
			col++
		}
		g, ok := glyphs[ch]
		if !ok {
			continue
		}
		for px := range len(g[0]) {
			for row := range 3 {
				top := g[row*2][px] == '#'
				bot := row*2+1 < 5 && g[row*2+1][px] == '#'
				var cell rune
				switch {
				case top && bot:
					cell = '█'
				case top:
					cell = '▀'
				case bot:
					cell = '▄'
				default:
					continue
				}
				for dx := range pw {
					r.Set(x+col+px*pw+dx, y+row, cell, color(col+px*pw+dx))
				}
			}
		}
		col += len(g[0]) * pw
	}
	return x + col
}
