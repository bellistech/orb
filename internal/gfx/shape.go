package gfx

// shape returns the tile mask for block, braille and line characters, which
// are drawn from geometry rather than the font so they tile seamlessly.
func shape(r rune) (glyph, bool) {
	var g glyph
	fill := func(rows uint8, from, to int) {
		for y := from; y < to; y++ {
			g[y] = rows
		}
	}
	switch {
	case r == '█':
		fill(0xFF, 0, Tile)
	case r >= '▉' && r <= '▏': // left eighths, ▉ = 7/8 down to ▏ = 1/8
		n := int('▏'-r) + 1
		fill(uint8(0xFF<<(Tile-n)), 0, Tile)
	case r == '▀':
		fill(0xFF, 0, Tile/2)
	case r == '▄':
		fill(0xFF, Tile/2, Tile)
	case r >= 0x2800 && r <= 0x28FF:
		// Braille: two columns of four dots, each dot 4x2 pixels so
		// adjacent dots join into solid areas.
		bits := r - 0x2800
		dots := [8]struct{ col, row int }{{0, 0}, {0, 1}, {0, 2}, {1, 0}, {1, 1}, {1, 2}, {0, 3}, {1, 3}}
		for i, d := range dots {
			if bits&(1<<i) == 0 {
				continue
			}
			mask := uint8(0xF0)
			if d.col == 1 {
				mask = 0x0F
			}
			g[d.row*2] |= mask
			g[d.row*2+1] |= mask
		}
	case r == '─':
		fill(0xFF, 3, 5)
	case r == '│':
		fill(0x18, 0, Tile)
	case r == '┄':
		g[4] = 0xCC
	case r >= '▁' && r <= '▇': // sparkline eighths, filled from the bottom
		fill(0xFF, Tile-int(r-'▁'+1)*Tile/8, Tile)
	case r == '├': // tree branches (bridge ports)
		fill(0x18, 0, Tile)
		g[3], g[4] = 0x1F, 0x1F
	case r == '└':
		fill(0x18, 0, 5)
		g[3], g[4] = 0x1F, 0x1F
	case r == '╭':
		g[3], g[4] = 0x07, 0x0F
		fill(0x18, 5, Tile)
	case r == '╮':
		g[3], g[4] = 0xE0, 0xF0
		fill(0x18, 5, Tile)
	case r == '╰':
		fill(0x18, 0, 3)
		g[3], g[4] = 0x0F, 0x07
	case r == '╯':
		fill(0x18, 0, 3)
		g[3], g[4] = 0xF0, 0xE0
	default:
		return g, false
	}
	return g, true
}

// frameRune reports runes that only draw panel borders; graphics mode
// replaces them with real windows.
func frameRune(r rune) bool {
	switch r {
	case '─', '│', '╭', '╮', '╰', '╯':
		return true
	}
	return false
}
