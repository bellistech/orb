package ui

import (
	"fmt"
	"sync"
	"time"

	"github.com/lucasb-eyer/go-colorful"

	"orb/internal/canvas"
)

// Cycle is the theme mode that holds each theme for CycleDwell, then
// cross-fades to the next over CycleFade, in an endless loop: after King
// Gizzard's Nonagon Infinity, whose last track runs into its first.
const Cycle = "nonagon"

const (
	// CycleDwell is long enough that a glance nearly always lands on a
	// settled palette (the fade is 5% of the loop) and panel hues stay put
	// while you work; all seven themes come round every ~7 minutes.
	CycleDwell = 60 * time.Second
	// CycleFade is long enough to read as a fade, not a flicker, and short
	// enough that the in-between colors rarely meet the eye.
	CycleFade = 3 * time.Second
	// FadeSteps is how many palettes a fade passes through, one every
	// 62.5 ms, about one per frame at the default 15 fps: no color moves by
	// more than a just-noticeable difference per step (TestFadeIsSmooth).
	// Fixed steps keep every palette one *Theme while it shows, cached by
	// the renderers like any theme, and the set of colors ever drawn finite.
	FadeSteps = 48
)

const cyclePeriod = CycleDwell + CycleFade

// ThemeNames lists the theme choices in t order: every theme, then Cycle.
func ThemeNames() []string {
	names := make([]string, 0, len(Themes)+1)
	for _, t := range Themes {
		names = append(names, t.Name)
	}
	return append(names, Cycle)
}

// ThemeSel is a theme choice: one theme, or the cycle and when it started.
type ThemeSel struct {
	i     int       // index into ThemeNames
	from  int       // cycle: the theme it starts on
	since time.Time // cycle: when it started on from
}

// SelectTheme returns the choice called name. The cycle starts at now,
// holding the default theme.
func SelectTheme(name string, now time.Time) (ThemeSel, bool) {
	if name == Cycle {
		s := ThemeSel{i: len(Themes), since: now}
		for i, t := range Themes {
			if t.Name == DefaultTheme {
				s.from = i
			}
		}
		return s, true
	}
	for i, t := range Themes {
		if t.Name == name {
			return ThemeSel{i: i}, true
		}
	}
	return ThemeSel{}, false
}

// Name is the choice's name, as -theme takes it.
func (s ThemeSel) Name() string {
	if s.i >= len(Themes) {
		return Cycle
	}
	return Themes[s.i].Name
}

// Next is the next choice in t order. The cycle, entered from the last
// theme, starts fading on to the first at once, so t always shows a change.
func (s ThemeSel) Next(now time.Time) ThemeSel {
	s.i = (s.i + 1) % (len(Themes) + 1)
	if s.i == len(Themes) {
		s.from, s.since = len(Themes)-1, now.Add(-CycleDwell)
	}
	return s
}

// Theme is the palette to draw with at now. Without fade (frames drawn
// only on samples) the cycle switches at the end of each dwell instead of
// cross-fading.
func (s ThemeSel) Theme(now time.Time, fade bool) *Theme {
	if s.i < len(Themes) {
		return Themes[s.i]
	}
	n := len(Themes)
	d := max(now.Sub(s.since), 0)
	i := (s.from + int(d/cyclePeriod%time.Duration(n))) % n
	u := d % cyclePeriod
	switch {
	case u < CycleDwell:
		return Themes[i]
	case !fade:
		return Themes[(i+1)%n]
	}
	return fadeStep(i, int((u-CycleDwell)*FadeSteps/CycleFade)+1)
}

// Wake is how long after now the palette next changes, for a front end
// that redraws only when something changed; 0 means never.
func (s ThemeSel) Wake(now time.Time) time.Duration {
	if s.i < len(Themes) {
		return 0
	}
	u := max(now.Sub(s.since), 0) % cyclePeriod
	if u < CycleDwell {
		return CycleDwell - u
	}
	const step = CycleFade / FadeSteps
	if u >= CycleDwell+(FadeSteps-1)*step {
		return cyclePeriod - u + CycleDwell // the last step is the next theme
	}
	return step - (u-CycleDwell)%step
}

// fades caches the in-between palettes, at most FadeSteps per theme, so a
// step is one *Theme for as long as it shows.
var fades struct {
	sync.Mutex
	m map[[2]int]*Theme
}

// fadeStep is step k (1..FadeSteps) of the fade from Themes[i] to the next.
func fadeStep(i, k int) *Theme {
	to := (i + 1) % len(Themes)
	if k >= FadeSteps {
		return Themes[to]
	}
	fades.Lock()
	defer fades.Unlock()
	if t := fades.m[[2]int{i, k}]; t != nil {
		return t
	}
	if fades.m == nil {
		fades.m = map[[2]int]*Theme{}
	}
	x := float64(k) / FadeSteps
	t := Blend(Themes[i], Themes[to], x*x*(3-2*x)) // ease in and out
	t.Name = fmt.Sprintf("%s %s>%s %d/%d", Cycle, Themes[i].Name, Themes[to].Name, k, FadeSteps)
	fades.m[[2]int{i, k}] = t
	return t
}

// blendPair is what a blended theme was mixed from, for Window.
type blendPair struct {
	a, b *Theme
	t    float64
}

// Blend returns the theme t of the way from a to b. Every color moves in
// OKLCh: lightness, chroma and hue each change evenly, so a fade between
// saturated themes stays saturated instead of going through gray as it
// does in RGB or OKLab (crimson's red reaches sylvan's green by way of
// amber, not mud).
func Blend(a, b *Theme, t float64) *Theme {
	t = max(0, min(1, t))
	m := func(x, y canvas.Color) canvas.Color { return MixLCh(x, y, t) }
	out := &Theme{
		Name: a.Name + ">" + b.Name,
		BG:   m(a.BG, b.BG), Panel: m(a.Panel, b.Panel), Track: m(a.Track, b.Track),
		FG: m(a.FG, b.FG), Dim: m(a.Dim, b.Dim), Faint: m(a.Faint, b.Faint),
		RX: m(a.RX, b.RX), TX: m(a.TX, b.TX),
		Host: m(a.Host, b.Host), CPU: m(a.CPU, b.CPU), Mem: m(a.Mem, b.Mem), GPU: m(a.GPU, b.GPU),
		Net: m(a.Net, b.Net), Disk: m(a.Disk, b.Disk), Temp: m(a.Temp, b.Temp), Proc: m(a.Proc, b.Proc),
		ramp:  make([]canvas.Color, rampSteps),
		blend: &blendPair{a, b, t},
	}
	for i := range out.ramp {
		out.ramp[i] = m(a.ramp[i], b.ramp[i])
	}
	return out
}

// MixLCh blends a toward b by t in OKLCh, the short way round the hue
// circle.
func MixLCh(a, b canvas.Color, t float64) canvas.Color {
	ca := colorful.Color{R: float64(a.R) / 255, G: float64(a.G) / 255, B: float64(a.B) / 255}
	cb := colorful.Color{R: float64(b.R) / 255, G: float64(b.G) / 255, B: float64(b.B) / 255}
	r, g, bl := ca.BlendOkLch(cb, t).Clamped().RGB255()
	return canvas.RGB(r, g, bl)
}
