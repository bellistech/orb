package gfx

// Pixel-art upscalers for character art (one byte per pixel, '.' clear):
// Scale2x and Scale3x (AdvMAME), which round off diagonal steps instead of
// just fattening each pixel, so a sprite drawn larger gains real detail.
// Pixels past the edge repeat the edge.

func artAt(art []string, x, y int) byte {
	y = max(0, min(y, len(art)-1))
	x = max(0, min(x, len(art[y])-1))
	return art[y][x]
}

// scale2x doubles art.
func scale2x(art []string) []string {
	out := make([]string, 0, 2*len(art))
	for y, line := range art {
		r0, r1 := make([]byte, 2*len(line)), make([]byte, 2*len(line))
		for x := range len(line) {
			p := line[x]
			a, b, c, d := artAt(art, x, y-1), artAt(art, x+1, y), artAt(art, x-1, y), artAt(art, x, y+1)
			e0, e1, e2, e3 := p, p, p, p
			if c == a && c != d && a != b {
				e0 = a
			}
			if a == b && a != c && b != d {
				e1 = b
			}
			if d == c && d != b && c != a {
				e2 = c
			}
			if b == d && b != a && d != c {
				e3 = d
			}
			r0[2*x], r0[2*x+1], r1[2*x], r1[2*x+1] = e0, e1, e2, e3
		}
		out = append(out, string(r0), string(r1))
	}
	return out
}

// scale3x triples art.
func scale3x(art []string) []string {
	out := make([]string, 0, 3*len(art))
	for y, line := range art {
		rows := [3][]byte{make([]byte, 3*len(line)), make([]byte, 3*len(line)), make([]byte, 3*len(line))}
		for x := range len(line) {
			A, B, C := artAt(art, x-1, y-1), artAt(art, x, y-1), artAt(art, x+1, y-1)
			D, E, F := artAt(art, x-1, y), line[x], artAt(art, x+1, y)
			G, H, I := artAt(art, x-1, y+1), artAt(art, x, y+1), artAt(art, x+1, y+1)
			pick := func(cond bool, v byte) byte {
				if cond {
					return v
				}
				return E
			}
			e := [9]byte{
				pick(D == B && D != H && B != F, D),
				pick((D == B && D != H && B != F && E != C) || (B == F && B != D && F != H && E != A), B),
				pick(B == F && B != D && F != H, F),
				pick((H == D && H != F && D != B && E != A) || (D == B && D != H && B != F && E != G), D),
				E,
				pick((B == F && B != D && F != H && E != I) || (F == H && F != B && H != D && E != C), F),
				pick(H == D && H != F && D != B, D),
				pick((F == H && F != B && H != D && E != G) || (H == D && H != F && D != B && E != I), H),
				pick(F == H && F != B && H != D, F),
			}
			for k := range 9 {
				rows[k/3][3*x+k%3] = e[k]
			}
		}
		out = append(out, string(rows[0]), string(rows[1]), string(rows[2]))
	}
	return out
}

// shade adds light and shadow to an upscaled sprite: a body pixel with
// clear sky or outline above it becomes hi, one with outline or clear
// below becomes lo, and a sprinkle of lo gives fur or feather texture.
// Only bytes in body are touched.
func shade(art []string, body string, hi, lo byte, texture uint32) []string {
	in := func(c byte) bool {
		for i := range len(body) {
			if body[i] == c {
				return true
			}
		}
		return false
	}
	out := make([]string, len(art))
	for y, line := range art {
		b := []byte(line)
		for x := range len(b) {
			if !in(line[x]) {
				continue
			}
			up, down := artAt(art, x, y-1), artAt(art, x, y+1)
			switch {
			case y > 0 && !in(up) && up != hi:
				b[x] = hi
			case y < len(art)-1 && !in(down) && down != lo:
				b[x] = lo
			case texture > 0 && hash(uint32(x)*31+7, uint32(y)*17+3)%texture == 0:
				b[x] = lo
			}
		}
		out[y] = string(b)
	}
	return out
}
