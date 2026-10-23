package gfx

import (
	"bytes"
	"image/png"
	"testing"
	"time"

	"orb/internal/canvas"
	"orb/internal/feeds"
	"orb/internal/sys"
	"orb/internal/ui"
)

func TestGlyphLandsInItsTile(t *testing.T) {
	th := ui.Themes[0]
	c := canvas.New(2, 1, th.BG)
	c.Full().Set(1, 0, '█', canvas.RGB(1, 2, 3))
	f := NewFrame(2*Tile, Tile)
	Rasterize(f, c, th, Anim{})
	for y := range Tile {
		for x := range 2 * Tile {
			want := rgb(th.BG)
			if x >= Tile {
				want = 0x010203
			}
			// The backdrop may hold a star; only check the block tile.
			if x >= Tile && f.Pix[y*f.W+x] != want {
				t.Fatalf("pixel %d,%d = %#06x, want %#06x", x, y, f.Pix[y*f.W+x], want)
			}
		}
	}
}

func TestEighthsAndBraille(t *testing.T) {
	g, _ := shape('▍') // 3/8
	if g[0] != 0xE0 {
		t.Fatalf("▍ row = %08b", g[0])
	}
	g, _ = shape(0x2800 | 0x01 | 0x80) // top-left and bottom-right dots
	if g[0] != 0xF0 || g[1] != 0xF0 || g[6] != 0x0F || g[7] != 0x0F || g[3] != 0 {
		t.Fatalf("braille = %v", g)
	}
}

func TestRasterizeRejectsMismatchedFrame(t *testing.T) {
	th := ui.Themes[0]
	c := canvas.New(4, 4, th.BG)
	f := NewFrame(5, 5)
	Rasterize(f, c, th, Anim{}) // must not panic or write
	Rasterize(nil, c, th, Anim{})
	Rasterize(f, nil, th, Anim{})
	for _, v := range f.Pix {
		if v != 0 {
			t.Fatal("wrote into a mismatched frame")
		}
	}
}

func TestWindowsAtAnySize(t *testing.T) {
	th := ui.Themes[0]
	for w := 1; w <= 12; w++ {
		for h := 1; h <= 12; h++ {
			c := canvas.New(14, 14, th.BG)
			ui.Frame(c.Full().Sub(1, 1, w, h), th, "T", "s", th.CPU)
			f := NewFrame(14*Tile, 14*Tile)
			Rasterize(f, c, th, Anim{T: 3.3})
		}
	}
}

func TestGrid(t *testing.T) {
	cols, rows, px, py, err := Grid(3440, 1440, 4)
	if err != nil || cols != 107 || rows != 45 || px != 8 || py != 0 {
		t.Fatalf("got %d %d %d %d %v", cols, rows, px, py, err)
	}
	for _, bad := range [][3]int{{3440, 1440, 0}, {3440, 1440, 9}, {0, 1440, 2}, {320, 200, 4}} {
		if _, _, _, _, err := Grid(bad[0], bad[1], bad[2]); err == nil {
			t.Errorf("Grid%v accepted", bad)
		}
	}
}

func TestWritePNG(t *testing.T) {
	f := NewFrame(3, 2)
	f.Pix[5] = 0xFF8000
	var b bytes.Buffer
	if err := WritePNG(&b, f, 2); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(&b)
	if err != nil {
		t.Fatal(err)
	}
	if r, g, bl, _ := img.At(5, 3).RGBA(); r>>8 != 0xFF || g>>8 != 0x80 || bl != 0 {
		t.Fatalf("scaled pixel = %d %d %d", r>>8, g>>8, bl>>8)
	}
	if err := WritePNG(&b, f, 0); err == nil {
		t.Fatal("scale 0 accepted")
	}
}

// One animated frame at the desk monitor size: render + rasterize.
func BenchmarkFrame(b *testing.B) {
	snap := sys.Demo(time.Unix(1_800_000_000, 0))
	th := ui.Themes[0]
	f := NewFrame(107*Tile, 45*Tile)
	for i := range b.N {
		Rasterize(f, ui.Render(107, 45, ui.View{Snap: snap}, th, true), th, Anim{T: float64(i) / 15})
	}
}

func TestQuarterScalesGlyph(t *testing.T) {
	g := font['A']
	for y := range 2 * Tile {
		for x := range 2 * Tile {
			q := uint8(1 + x/Tile + 2*(y/Tile))
			big := quarter(g, q)[y%Tile]&(0x80>>(x%Tile)) != 0
			small := g[y/2]&(0x80>>(x/2)) != 0
			if big != small {
				t.Fatalf("pixel %d,%d: quarter=%v source=%v", x, y, big, small)
			}
		}
	}
	if quarter(g, 0) != (glyph{}) || quarter(g, 5) != (glyph{}) {
		t.Fatal("bad quadrant should be empty")
	}
}

// The incremental renderer (tile diffing, overlays, stars) must always
// produce exactly the frame a fresh full render would: no trails, no
// stale tiles, whatever the sequence of frames.
func TestIncrementalMatchesFullRender(t *testing.T) {
	start := time.Unix(1_800_000_000, 0)
	fv := feeds.Demo(start)
	steps := []float64{0, 0.07, 0.5, 1.3, 2.9, 3.0, 4.4, 5.05, 7.7, 8.0, 9.1, 12.5, 13.1, 20.2}
	for _, th := range []*ui.Theme{ui.Themes[0], ui.Themes[len(ui.Themes)-1]} {
		for _, page := range []ui.Page{ui.Dash, ui.News, ui.Net, ui.Links} {
			// One renderer across consecutive frames, as the run loop does.
			inc := NewRenderer()
			for i, step := range steps {
				now := start.Add(time.Duration(step * float64(time.Second)))
				c := ui.Render(107, 45, ui.View{Page: page, Snap: sys.Demo(now), Feeds: fv, Now: now}, th, true)
				a := Anim{T: 100 + step}
				inc.Draw(c, th, a)
				full := NewRenderer()
				full.Draw(c, th, a)
				got, want := inc.Frame().Pix, full.Frame().Pix
				for p := range want {
					if got[p] != want[p] {
						t.Fatalf("%s %v step %d (t=%.2f): pixel %d,%d = %#06x, full render %#06x",
							th.Name, page, i, step, p%inc.Frame().W, p/inc.Frame().W, got[p], want[p])
					}
				}
			}
		}
	}
}

func TestOrbStaysInItsRect(t *testing.T) {
	th := ui.Themes[0]
	c := canvas.New(20, 10, th.BG)
	c.Full().Sub(2, 0, 4, 5).AddOrb(th.Host)
	r := NewRenderer()
	r.Draw(c, th, Anim{T: 5})
	bg := rgb(th.BG)
	for y := range r.Frame().H {
		for x := range r.Frame().W {
			in := x >= 2*Tile && x < 6*Tile && y < 5*Tile
			if !in && r.Frame().Pix[y*r.Frame().W+x] != bg && hash(uint32(x), uint32(y))%997 != 0 {
				t.Fatalf("orb leaked to %d,%d", x, y)
			}
		}
	}
	if r.Frame().Pix[2*Tile*1+16+20*r.Frame().W] == bg {
		t.Fatal("orb not drawn")
	}
}
