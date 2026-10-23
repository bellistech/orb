package gfx

import (
	"testing"
	"time"

	"orb/internal/canvas"
	"orb/internal/feeds"
	"orb/internal/ui"
)

var allMoods = []ui.Mood{ui.Nap, ui.Sit, ui.Trot, ui.Sprint, ui.Pant, ui.Bark, ui.Play}

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
		// Every frame stands on the ground (the bottom row is its
		// outline), but the pounce.
		if art[dogH-1] == "...................." && dogFrame(f) != dogLeap {
			t.Fatalf("frame %d floats", f)
		}
	}
	for f, art := range ballArt {
		for y, row := range art {
			if len(row) != ballW {
				t.Fatalf("ball %d row %d is %d wide, want %d", f, y, len(row), ballW)
			}
			for x := range len(row) {
				if b := row[x]; b != '.' && (b >= 128 || !pal.ok[b]) {
					t.Fatalf("ball %d (%d,%d): unknown art byte %q", f, x, y, b)
				}
			}
		}
		if f > 0 && art == ballArt[f-1] {
			t.Fatalf("ball %d does not turn", f)
		}
	}
}

// Poses are a pure function of mood, pen and time, and keep the sprite and
// the ball inside the pen.
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
			// The ball rests on the ground row's shadow, never below it.
			if b := p.ball; b.x < pr.x0 || b.x+ballW > pr.x1 || b.y < pr.y0 || b.y+ballW > pr.y1-1 {
				t.Fatalf("%v at %.3f: ball %d,%d leaves pen %+v", m, T, b.x, b.y, pr)
			}
			if hide := m == ui.Bark || m == ui.Trot || m == ui.Sprint; p.ball.hide != hide {
				t.Fatalf("%v at %.3f: ball hidden %v", m, T, p.ball.hide)
			}
			if p.x != first.x {
				moved = true
			}
		}
		// Sitting plays ball every so often.
		if runs := m == ui.Trot || m == ui.Sprint || m == ui.Sit || m == ui.Play; moved != runs {
			t.Errorf("%v: moved=%v, want %v", m, moved, runs)
		}
	}
	// A pen too small to run in leaves the dog at home, not off the edge.
	tiny := rect{0, 0, dogW + 4, 3 * Tile}
	if p := dogPoseAt(ui.Sprint, tiny, 3.3); p.x != tiny.x0+2 {
		t.Fatalf("tiny pen: x = %d", p.x)
	}
	for i := range 200 {
		if p := dogPoseAt(ui.Play, tiny, float64(i)*0.04); p.x != tiny.x0+2 || p.ball.x < tiny.x0 {
			t.Fatalf("tiny pen at %.2f: dog %d, ball %d", float64(i)*0.04, p.x, p.ball.x)
		}
	}
}

// A bout of ball play: the ball leaves home and comes back in the dog's
// mouth, and the bout starts and ends as a plain sit (dog home, facing
// out, ball at its feet) in the narrowest pen, at the desk and on a wide
// screen, so nothing jumps when Sit and Play trade places.
func TestDogPlayBout(t *testing.T) {
	for _, cells := range []int{12, 30, 60} {
		pr := rect{50 * Tile, 0, (50 + cells) * Tile, 3 * Tile}
		hi := pr.x1 - dogW - 2
		home := dogPoseAt(ui.Nap, pr, 0).ball
		rest := func(T float64) {
			t.Helper()
			p := dogPoseAt(ui.Sit, pr, T)
			if p.x != hi || p.y != pr.y1-1-dogH || p.ball != home {
				t.Fatalf("%d cells at %.2f: dog %d,%d ball %+v, want %d and %+v", cells, T, p.x, p.y, p.ball, hi, home)
			}
		}
		rest(15.99) // just before the bout
		rest(16.01) // its first bow
		rest(23.99) // its end
		rest(24.01) // after
		far, held, leapt := 0, false, false
		for i := range 800 {
			T := 16 + float64(i)*0.01
			if m, _ := ui.DogAct(ui.Sit, T); m != ui.Play {
				t.Fatalf("not playing at %.2f", T)
			}
			p := dogPoseAt(ui.Sit, pr, T)
			far = max(far, home.x-p.ball.x)
			held = held || p.ball.held
			leapt = leapt || p.frame == dogLeap
		}
		if far < 20 || !held || !leapt {
			t.Fatalf("%d cells: ball went %d px, held %v, pounced %v", cells, far, held, leapt)
		}
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
			var ovs []rect
			for _, o := range r.dog(c.Pets[0], th, a) {
				ovs = append(ovs, o.r)
			}
			drawn := 0
			for y := range r.Frame().H {
				for x := range r.Frame().W {
					i := y*r.Frame().W + x
					if r.Frame().Pix[i] == bare.Frame().Pix[i] {
						continue
					}
					drawn++
					in := false
					for _, ov := range ovs {
						in = in || x >= ov.x0 && x < ov.x1 && y >= ov.y0 && y < ov.y1
					}
					if !in {
						t.Fatalf("%v at %.2f: drew %d,%d outside %+v", m, a.T, x, y, ovs)
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
// a full render through a sequence that walks every mood, then a whole bout
// of ball play cut into by a trot and a bark (the ball is put away and
// comes back).
// It holds under a live sky (rain and snow falling around the z's that
// rise over the header's top edge) and through a nonagon cross-fade.
func TestDogIncrementalMatchesFullRender(t *testing.T) {
	t0 := time.Date(2026, 10, 23, 21, 0, 0, 0, time.UTC) // night
	sel, _ := ui.SelectTheme(ui.Cycle, t0)
	kinds := []feeds.Kind{feeds.Rain, feeds.Snow, feeds.Storm, feeds.Clear}
	inc := NewRenderer()
	for i := range 220 {
		m := allMoods[(i/7)%len(allMoods)]
		T := 50 + float64(i)*0.071
		if i >= 120 { // a bout: 15.6 s to 24.5 s on the clock
			m, T = ui.Sit, 15.6+float64(i-120)*0.09
			switch {
			case i >= 160 && i < 164:
				m = ui.Trot
			case i >= 180 && i < 185:
				m = ui.Bark
			}
		}
		d := ui.CycleDwell - 2*time.Second + time.Duration(i%120)*ui.CycleFade/40
		now := t0.Add(d)
		th := sel.Theme(now, true)
		c := canvas.New(143, 8, th.BG)
		c.Full().Sub(0, 0, 143, 5).AddWindow(th.Host)
		c.Full().Sub(92, 0, 30, 3).AddPet(th.Host, uint8(m), "cpu 96°")
		a := Anim{T: T, Sky: Sky{Now: now, Kind: kinds[(i/30)%len(kinds)]}}
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
