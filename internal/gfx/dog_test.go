package gfx

import (
	"testing"
	"time"

	"orb/internal/canvas"
	"orb/internal/feeds"
	"orb/internal/ui"
)

var allMoods = []ui.Mood{ui.Nap, ui.Sit, ui.Trot, ui.Sprint, ui.Pant, ui.Bark}

func TestDogArtWellFormed(t *testing.T) {
	pal := newDogPalette(ui.Themes[0], ui.Themes[0].Host)
	for f, art := range dogArt {
		opaque := 0
		for y, row := range art {
			if len(row) != dogW {
				t.Fatalf("frame %d row %d is %d wide, want %d", f, y, len(row), dogW)
			}
			for x := range len(row) {
				b := row[x]
				if b != '.' && (b >= 128 || !pal.ok[b]) {
					t.Fatalf("frame %d (%d,%d): unknown art byte %q", f, x, y, b)
				}
				if b != '.' {
					opaque++
				}
			}
		}
		if opaque < 60 {
			t.Fatalf("frame %d is nearly empty", f)
		}
		// Every frame stands on the ground: the bottom row is its outline.
		if art[dogH-1] == "...................." {
			t.Fatalf("frame %d floats", f)
		}
	}
}

// Poses are a pure function of mood, pen and time, and keep the sprite
// inside the pen.
func TestDogPoseStaysInPen(t *testing.T) {
	pr := rect{92 * Tile, 0, 122 * Tile, 3 * Tile}
	for _, m := range allMoods {
		moved := false
		first := dogPoseAt(m, pr, 0)
		for i := range 2000 {
			T := float64(i) * 0.037
			p := dogPoseAt(m, pr, T)
			if p != dogPoseAt(m, pr, T) {
				t.Fatalf("%v: pose at %.3f not deterministic", m, T)
			}
			if p.x < pr.x0 || p.x+dogW > pr.x1 || p.y < pr.y0 || p.y+dogH > pr.y1 {
				t.Fatalf("%v at %.3f: sprite %d,%d leaves pen %+v", m, T, p.x, p.y, pr)
			}
			if p.x != first.x {
				moved = true
			}
		}
		if runs := m == ui.Trot || m == ui.Sprint; moved != runs {
			t.Errorf("%v: moved=%v, want %v", m, moved, runs)
		}
	}
	// A pen too small to run in leaves the dog at home, not off the edge.
	tiny := rect{0, 0, dogW + 4, 3 * Tile}
	if p := dogPoseAt(ui.Sprint, tiny, 3.3); p.x != tiny.x0+2 {
		t.Fatalf("tiny pen: x = %d", p.x)
	}
}

// The dog and its effects (z's, dust, sweat, bubble) draw only inside the
// overlay rect, so the tile restore underneath erases them next frame.
func TestDogStaysInItsOverlay(t *testing.T) {
	th := ui.Themes[1]
	for _, m := range allMoods {
		c := canvas.New(143, 8, th.BG)
		c.Full().Sub(92, 0, 30, 3).AddPet(th.Host, uint8(m), "disk 97%")
		for i := range 40 {
			a := Anim{T: 1 + float64(i)*0.13}
			r := NewRenderer()
			bare := NewRenderer()
			plain := canvas.New(143, 8, th.BG)
			bare.Draw(plain, th, a)
			r.Draw(c, th, a)
			ov := r.dog(c.Pets[0], th, a).r
			drawn := 0
			for y := range r.Frame().H {
				for x := range r.Frame().W {
					i := y*r.Frame().W + x
					if r.Frame().Pix[i] == bare.Frame().Pix[i] {
						continue
					}
					drawn++
					if x < ov.x0 || x >= ov.x1 || y < ov.y0 || y >= ov.y1 {
						t.Fatalf("%v at %.2f: drew %d,%d outside %+v", m, a.T, x, y, ov)
					}
				}
			}
			if drawn < 60 {
				t.Fatalf("%v at %.2f: dog not drawn (%d px)", m, a.T, drawn)
			}
		}
	}
}

// Mood changes and motion leave no trails: the incremental renderer equals
// a full render through a sequence that walks every mood.
// It holds under a live sky (rain and snow falling around the z's that
// rise over the header's top edge) and through a nonagon cross-fade.
func TestDogIncrementalMatchesFullRender(t *testing.T) {
	t0 := time.Date(2026, 10, 23, 21, 0, 0, 0, time.UTC) // night
	sel, _ := ui.SelectTheme(ui.Cycle, t0)
	kinds := []feeds.Kind{feeds.Rain, feeds.Snow, feeds.Storm, feeds.Clear}
	inc := NewRenderer()
	for i := range 120 {
		m := allMoods[(i/7)%len(allMoods)]
		d := ui.CycleDwell - 2*time.Second + time.Duration(i)*ui.CycleFade/40
		now := t0.Add(d)
		th := sel.Theme(now, true)
		c := canvas.New(143, 8, th.BG)
		c.Full().Sub(0, 0, 143, 5).AddWindow(th.Host)
		c.Full().Sub(92, 0, 30, 3).AddPet(th.Host, uint8(m), "cpu 96°")
		a := Anim{T: 50 + float64(i)*0.071, Sky: Sky{Now: now, Kind: kinds[(i/30)%len(kinds)]}}
		inc.Draw(c, th, a)
		full := NewRenderer()
		full.Draw(c, th, a)
		got, want := inc.Frame().Pix, full.Frame().Pix
		for p := range want {
			if got[p] != want[p] {
				t.Fatalf("%s frame %d (%v): pixel %d,%d = %#06x, full %#06x", th.Name, i, m, p%inc.Frame().W, p/inc.Frame().W, got[p], want[p])
			}
		}
	}
}

func TestAlarmInkReadsOnCream(t *testing.T) {
	for _, th := range ui.Themes {
		c := alarmInk(th)
		if l := 0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B); l >= 150 {
			t.Errorf("%s: alarm ink %v too light (%.0f)", th.Name, c, l)
		}
	}
}
