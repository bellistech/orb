package gfx

import (
	"strings"
	"testing"

	"orb/internal/ui"
)

// play runs an intro on a virtual 15 fps clock until it is over, calling
// frame after each Draw, and returns when it ended.
func play(t *testing.T, in *Intro, w, h int, th *ui.Theme, ready func(el float64) bool, frame func(el float64, spans []Span)) float64 {
	t.Helper()
	for i := 0; i < 15*60; i++ {
		el := float64(i) / 15
		if in.Over(el, ready(el)) {
			return el
		}
		spans := in.Draw(w, h, th, el)
		if frame != nil {
			frame(el, spans)
		}
	}
	t.Fatal("intro never ended")
	return 0
}

func TestIntroTimeline(t *testing.T) {
	th := ui.Themes[0]
	nominal := splashLen + titleMin + introFade
	// Samples arrive at the default 1 s interval: the intro runs its
	// nominal length, about 4 s.
	end := play(t, &Intro{}, 320, 120, th, func(el float64) bool { return el >= 1 }, nil)
	if end < nominal || end > nominal+0.1 {
		t.Fatalf("ended at %.2fs, want %.2fs", end, nominal)
	}
	if nominal < 3 || nominal > 5 {
		t.Fatalf("nominal length %.2fs, want 3..5s", nominal)
	}
	// A slow first sample: the title card waits for it.
	end = play(t, &Intro{}, 320, 120, th, func(el float64) bool { return el >= 6 }, nil)
	if end < 6+introFade || end > 6+introFade+0.1 {
		t.Fatalf("slow sample: ended at %.2fs, want %.2fs", end, 6+introFade)
	}
	// None at all: it gives up after titleWait.
	end = play(t, &Intro{}, 320, 120, th, func(float64) bool { return false }, nil)
	if want := nominal + titleWait; end < want || end > want+0.1 {
		t.Fatalf("no sample: ended at %.2fs, want %.2fs", end, want)
	}
}

// The spans Draw reports must carry every changed pixel: a screen updated
// only by them always equals the composed frame. Frames before the fade
// are a pure function of time.
func TestIntroSpansTrackTheFrame(t *testing.T) {
	th := ui.Themes[1]
	w, h := 1144, 480
	screen := NewFrame(w, h)
	in := &Intro{Sub: "dev · gizmo"}
	n := 0
	play(t, in, w, h, th, func(el float64) bool { return el >= 1 }, func(el float64, spans []Span) {
		f := in.Frame()
		for _, sp := range spans {
			copy(screen.Pix[sp.Y*w+sp.X0:sp.Y*w+sp.X1], f.Pix[sp.Y*w+sp.X0:sp.Y*w+sp.X1])
		}
		for p := range f.Pix {
			if screen.Pix[p] != f.Pix[p] {
				t.Fatalf("t=%.2f: pixel %d,%d on screen %#06x, frame %#06x", el, p%w, p/w, screen.Pix[p], f.Pix[p])
			}
		}
		if n++; n%7 == 0 && in.end == 0 {
			fresh := &Intro{Sub: in.Sub}
			fresh.Draw(w, h, th, el)
			for p := range f.Pix {
				if fresh.Frame().Pix[p] != f.Pix[p] {
					t.Fatalf("t=%.2f: frame depends on history at %d,%d", el, p%w, p/w)
				}
			}
		}
	})
	// A reset (console switched back) reports every row.
	in.Reset()
	if spans := in.Draw(w, h, th, 3); len(spans) != h {
		t.Fatalf("after Reset: %d spans, want %d", len(spans), h)
	}
}

// count returns how many pixels in rows [y0, y1) differ from v.
func count(f *Frame, y0, y1 int, v uint32) int {
	n := 0
	for _, p := range f.Pix[y0*f.W : y1*f.W] {
		if p != v {
			n++
		}
	}
	return n
}

func TestIntroScenes(t *testing.T) {
	th := ui.Themes[1]
	bg := rgb(th.BG)
	w, h := 1144, 480
	at := func(el float64) *Frame {
		in := &Intro{Sub: "dev · gizmo"}
		in.Draw(w, h, th, el)
		return in.Frame()
	}
	if n := count(at(0), 0, h, bg); n != 0 {
		t.Fatalf("splash starts from a blank screen, %d pixels lit", n)
	}
	// The wordmark sits mid screen.
	if n := count(at(0.8), h*2/5, h*3/5, bg); n < 1000 {
		t.Fatalf("splash: %d pixels lit mid screen", n)
	}
	// The title card: the orb at the top, PRESS START blinking near the
	// bottom (on 0.6 s of every second from 1.2 s in).
	title := at(splashLen + 1.5)
	if title.Pix[h*30/100*w+w/2] == bg {
		t.Fatal("no orb")
	}
	band := func(f *Frame) int { return count(f, h*86/100, h*86/100+16, bg) }
	on, off := band(title), band(at(splashLen+2))
	if on < 100 || on < off+100 {
		t.Fatalf("PRESS START: %d pixels lit when on, %d when off", on, off)
	}
	// The fade out ends on a blank screen, where the dashboard takes over.
	in := &Intro{}
	el := 0.0
	for ; !in.Over(el, true); el += 1.0 / 15 {
		in.Draw(w, h, th, el)
	}
	in.Draw(w, h, th, el)
	if n := count(in.Frame(), 0, h, bg); n != 0 {
		t.Fatalf("fade out ends with %d pixels lit", n)
	}
}

// Any screen orb accepts, any subtitle: no panic, and every pixel written
// stays inside the frame (Frame.set and fill clip).
func TestIntroAnySize(t *testing.T) {
	th := ui.Themes[0]
	for _, sz := range [][2]int{{320, 96}, {321, 97}, {640, 360}, {1144, 480}, {3440, 1440}, {320, 2000}} {
		for _, sub := range []string{"", "dev · gizmo", strings.Repeat("\x00é·w", 300)} {
			in := &Intro{Sub: sub}
			for _, el := range []float64{0, 0.3, 0.9, 1.69, 1.7, 2.0, 2.25, 3.0, 4.0, 9} {
				in.Over(el, true)
				in.Draw(sz[0], sz[1], th, el)
			}
		}
	}
}

// One intro frame on the 3440x1440 desk monitor at scale 3, through the
// busiest part of the title card.
func BenchmarkIntroFrame(b *testing.B) {
	th := ui.Themes[1]
	in := &Intro{Sub: "dev · gizmo"}
	for i := range b.N {
		in.Draw(1144, 480, th, splashLen+0.9+float64(i%30)/15)
	}
}
