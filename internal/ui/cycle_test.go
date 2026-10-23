package ui

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/lucasb-eyer/go-colorful"

	"orb/internal/canvas"
)

func okLab(c canvas.Color) (l, a, b float64) {
	return colorful.Color{R: float64(c.R) / 255, G: float64(c.G) / 255, B: float64(c.B) / 255}.OkLab()
}

// deltaE is the distance between two colors in OKLab: ~0.02 is about the
// smallest difference the eye notices.
func deltaE(x, y canvas.Color) float64 {
	l1, a1, b1 := okLab(x)
	l2, a2, b2 := okLab(y)
	return math.Sqrt((l1-l2)*(l1-l2) + (a1-a2)*(a1-a2) + (b1-b2)*(b1-b2))
}

// colors lists every color of a theme a renderer uses, ramp and the
// window colors for every accent included.
func colors(t *Theme) []canvas.Color {
	accents := []canvas.Color{t.Host, t.CPU, t.Mem, t.GPU, t.Net, t.Disk, t.Temp, t.Proc}
	out := append([]canvas.Color{t.BG, t.Panel, t.Track, t.FG, t.Dim, t.Faint, t.RX, t.TX}, accents...)
	for _, a := range accents {
		top, bot, edge := t.Window(a)
		out = append(out, top, bot, edge)
	}
	return append(out, t.ramp...)
}

func TestThemeNames(t *testing.T) {
	names := ThemeNames()
	if len(names) != len(Themes)+1 || names[len(names)-1] != Cycle {
		t.Fatalf("ThemeNames = %v, want every theme then %s", names, Cycle)
	}
	for i, n := range names {
		if slices.Index(names, n) != i {
			t.Errorf("theme %q listed twice", n)
		}
		s, ok := SelectTheme(n, time.Unix(0, 0))
		if !ok || s.Name() != n {
			t.Errorf("SelectTheme(%q) = %q, %v", n, s.Name(), ok)
		}
	}
	for _, n := range []string{DefaultTheme, "sylvan"} {
		if ThemeByName(n) == nil {
			t.Errorf("theme %q missing", n)
		}
	}
	if _, ok := SelectTheme("forest", time.Unix(0, 0)); ok {
		t.Error("unknown theme accepted")
	}
}

// t visits every choice in order and wraps.
func TestThemeRotation(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	s, _ := SelectTheme(Themes[0].Name, now)
	names := ThemeNames()
	for i := range 2 * len(names) {
		if got, want := s.Name(), names[i%len(names)]; got != want {
			t.Fatalf("after %d t presses: %q, want %q", i, got, want)
		}
		s = s.Next(now)
	}
}

func TestBlendEndpoints(t *testing.T) {
	for i, a := range Themes {
		b := Themes[(i+1)%len(Themes)]
		for _, end := range []struct {
			t    float64
			want *Theme
		}{{0, a}, {1, b}, {-1, a}, {2, b}} {
			got, want := colors(Blend(a, b, end.t)), colors(end.want)
			for k := range want {
				if deltaE(got[k], want[k]) > 0.004 { // RGB rounding
					t.Fatalf("Blend(%s, %s, %v) color %d = %v, want %v", a.Name, b.Name, end.t, k, got[k], want[k])
				}
			}
		}
	}
}

// Along a fade every color moves away from where it started and toward
// where it ends, never back, and no step is big enough to notice.
func TestFadeIsSmooth(t *testing.T) {
	const eps = 0.006 // 8-bit rounding and gamut clamping
	worst := 0.0
	for i, a := range Themes {
		b := Themes[(i+1)%len(Themes)]
		ca, cb := colors(a), colors(b)
		prev := ca
		for k := 1; k <= FadeSteps; k++ {
			cur := colors(fadeStep(i, k))
			for c := range cur {
				if deltaE(cur[c], ca[c])+eps < deltaE(prev[c], ca[c]) || deltaE(cur[c], cb[c]) > deltaE(prev[c], cb[c])+eps {
					t.Fatalf("%s>%s step %d color %d: %v after %v is not monotone", a.Name, b.Name, k, c, cur[c], prev[c])
				}
				worst = max(worst, deltaE(cur[c], prev[c]))
			}
			prev = cur
		}
	}
	if worst > 0.025 {
		t.Errorf("biggest fade step dE %.3f, want under 0.025", worst)
	}
}

func TestCycleTiming(t *testing.T) {
	t0 := time.Unix(1_800_000_000, 0)
	s, _ := SelectTheme(Cycle, t0)
	def := ThemeByName(DefaultTheme)
	start := slices.Index(Themes, def)
	next := Themes[(start+1)%len(Themes)]
	at := func(d time.Duration, fade bool) *Theme { return s.Theme(t0.Add(d), fade) }

	for _, d := range []time.Duration{-time.Hour, 0, time.Second, CycleDwell - 1} {
		if got := at(d, true); got != def {
			t.Errorf("at %v: %s, want %s", d, got.Name, def.Name)
		}
	}
	mid := at(CycleDwell+CycleFade/2, true)
	if slices.Contains(Themes, mid) {
		t.Errorf("mid-fade shows %s, want a blend", mid.Name)
	}
	if mid != at(CycleDwell+CycleFade/2+time.Millisecond, true) {
		t.Error("one fade step should be one palette")
	}
	if got := at(cyclePeriod, true); got != next {
		t.Errorf("after the fade: %s, want %s", got.Name, next.Name)
	}
	// Static frames switch at the end of the dwell.
	if got := at(CycleDwell, false); got != next {
		t.Errorf("static at the end of the dwell: %s, want %s", got.Name, next.Name)
	}
	// Endless: every theme in turn, then round again.
	for k := range 2*len(Themes) + 1 {
		want := Themes[(start+k)%len(Themes)]
		if got := at(time.Duration(k)*cyclePeriod+time.Second, true); got != want {
			t.Errorf("loop %d: %s, want %s", k, got.Name, want.Name)
		}
	}
	if got := at(1000*24*time.Hour, true); !slices.Contains(Themes, got) && got.blend == nil {
		t.Error("far future has no theme")
	}

	// Wake lands on the next palette change.
	if w := s.Wake(t0.Add(10 * time.Second)); w != CycleDwell-10*time.Second {
		t.Errorf("Wake in dwell = %v", w)
	}
	for d := CycleDwell; d < cyclePeriod; d += 37 * time.Millisecond {
		w := s.Wake(t0.Add(d))
		if w <= 0 || w > CycleFade/FadeSteps && at(d, true) != next {
			t.Fatalf("Wake at %v = %v", d, w)
		}
		if at(d+w-1, true) != at(d, true) || at(d+w, true) == at(d, true) {
			t.Fatalf("at %v the palette should change exactly %v later", d, w)
		}
	}
	if fixed, _ := SelectTheme("sylvan", t0); fixed.Wake(t0) != 0 || fixed.Theme(t0.Add(time.Hour), true) != ThemeByName("sylvan") {
		t.Error("a fixed theme changed")
	}
}

// t into the cycle fades from the last theme to the first straight away.
func TestCycleEnteredByKey(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	s, _ := SelectTheme(Themes[len(Themes)-1].Name, now)
	s = s.Next(now)
	if s.Name() != Cycle {
		t.Fatalf("t from the last theme gave %s", s.Name())
	}
	if th := s.Theme(now, true); th.blend == nil || th.blend.a != Themes[len(Themes)-1] || th.blend.b != Themes[0] {
		t.Errorf("entering the cycle shows %s, want a fade to %s", th.Name, Themes[0].Name)
	}
	if th := s.Theme(now.Add(CycleFade), true); th != Themes[0] {
		t.Errorf("after the fade: %s, want %s", th.Name, Themes[0].Name)
	}
	if th := s.Theme(now, false); th != Themes[0] {
		t.Errorf("static: %s, want %s", th.Name, Themes[0].Name)
	}
}
