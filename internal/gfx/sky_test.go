package gfx

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"orb/internal/feeds"
	"orb/internal/sys"
	"orb/internal/ui"
)

// cst is a zone without daylight saving time: solar noon at 12:00.
var cst = time.FixedZone("CST", -6*3600)

func at(month time.Month, day, hour, min int) time.Time {
	return time.Date(2026, month, day, hour, min, 0, 0, cst)
}

func TestSunTimesFollowTheSeasons(t *testing.T) {
	sr, ss := sunTimes(at(6, 21, 12, 0))
	wr, ws := sunTimes(at(12, 21, 12, 0))
	er, es := sunTimes(at(3, 20, 12, 0))
	if l := ss - sr; l < 14 || l > 14.8 {
		t.Errorf("summer day %.2f h", l)
	}
	if l := ws - wr; l < 9.4 || l > 10.2 {
		t.Errorf("winter day %.2f h", l)
	}
	if l := es - er; l < 11.7 || l > 12.3 {
		t.Errorf("equinox day %.2f h", l)
	}
	for _, d := range [][2]float64{{sr, ss}, {wr, ws}, {er, es}} {
		if mid := (d[0] + d[1]) / 2; mid < 11.99 || mid > 12.01 {
			t.Errorf("solar noon at %.2f", mid)
		}
	}
	// Daylight saving time moves the clock, not the sun.
	dst, _ := time.LoadLocation("America/Chicago")
	if dst != nil {
		r, s := sunTimes(time.Date(2026, 6, 21, 12, 0, 0, 0, dst))
		if mid := (r + s) / 2; mid < 12.99 || mid > 13.01 {
			t.Errorf("DST solar noon at %.2f", mid)
		}
	}
}

func TestSkyPhaseByTimeOfDay(t *testing.T) {
	// 2026-10-23 at 35N: sunrise ~6:33, sunset ~17:27.
	for _, c := range []struct {
		h, m int
		want phase
	}{
		{0, 0, night}, {3, 30, night}, {5, 30, night},
		{6, 0, dawn}, {6, 40, dawn}, {6, 50, dawn},
		{8, 0, day}, {12, 0, day}, {16, 30, day},
		{17, 15, dusk}, {17, 40, dusk}, {18, 0, dusk},
		{19, 0, night}, {23, 59, night},
	} {
		if got := phaseAt(at(10, 23, c.h, c.m)); got != c.want {
			t.Errorf("%02d:%02d: %v, want %v", c.h, c.m, got, c.want)
		}
	}
	if l, g := daylight(at(10, 23, 12, 0)); l != 1 || g != 0 {
		t.Errorf("noon light %v glow %v", l, g)
	}
	if l, _ := daylight(at(10, 23, 1, 0)); l != 0 {
		t.Errorf("night light %v", l)
	}
	if _, g := daylight(at(10, 23, 17, 27)); g < 0.95 {
		t.Errorf("sunset glow %v", g)
	}
}

func TestSunAndMoonArc(t *testing.T) {
	const w, h = 1144, 480
	frac, sun := bodyAt(at(10, 23, 12, 0))
	if !sun || frac < 0.49 || frac > 0.51 {
		t.Fatalf("noon: frac %v sun %v", frac, sun)
	}
	x, y := arc(frac, w, h, 40)
	if x < w/2-3 || x > w/2+3 || y != 40 {
		t.Errorf("noon sun at %d,%d, want top center", x, y)
	}
	// Morning sun is left of center and low; afternoon right.
	fm, _ := bodyAt(at(10, 23, 8, 0))
	fa, _ := bodyAt(at(10, 23, 16, 0))
	xm, ym := arc(fm, w, h, 40)
	xa, ya := arc(fa, w, h, 40)
	if xm >= w/2 || xa <= w/2 || ym <= y || ya <= y {
		t.Errorf("morning %d,%d afternoon %d,%d noon %d,%d", xm, ym, xa, ya, x, y)
	}
	// Sunrise: just under the bottom left corner.
	rise, _ := sunTimes(at(10, 23, 0, 0))
	fr, _ := bodyAt(at(10, 23, int(rise), int((rise-float64(int(rise)))*60)+1))
	if xr, yr := arc(fr, w, h, 40); xr > 20 || yr < h-bodyR {
		t.Errorf("sunrise at %d,%d", xr, yr)
	}
	// Midnight: the moon, highest near the middle of the night.
	fn, sun := bodyAt(at(10, 23, 0, 0))
	if sun || fn < 0.45 || fn > 0.6 {
		t.Errorf("midnight: frac %v sun %v", fn, sun)
	}
	if _, sun := bodyAt(at(10, 23, 22, 0)); sun {
		t.Error("sun at 22:00")
	}
}

func TestMoonPhase(t *testing.T) {
	for _, c := range []struct {
		t    time.Time
		want float64
	}{
		{time.Date(2000, 1, 6, 18, 14, 0, 0, time.UTC), 0},     // new
		{time.Date(2024, 4, 8, 18, 21, 0, 0, time.UTC), 0},     // new (eclipse)
		{time.Date(2024, 4, 23, 23, 49, 0, 0, time.UTC), 0.5},  // full
		{time.Date(2026, 10, 26, 4, 12, 0, 0, time.UTC), 0.5},  // full
		{time.Date(2026, 10, 29, 16, 36, 0, 0, time.UTC), 0.6}, // ~last quarter is 11-01
	} {
		got := moonPhase(c.t)
		d := min(abs(got-c.want), 1-abs(got-c.want))
		if d > 0.12 {
			t.Errorf("%v: phase %.3f, want ~%.2f", c.t, got, c.want)
		}
	}
}

func abs(x float64) float64 { return max(x, -x) }

func TestWeatherEffects(t *testing.T) {
	for _, c := range []struct {
		k    feeds.Kind
		want fx
	}{
		{feeds.Unknown, fx{}},
		{feeds.Clear, fx{}},
		{feeds.PartlyCloudy, fx{clouds: 0.35, overcast: 0.1}},
		{feeds.Cloudy, fx{clouds: 1, overcast: 0.6}},
		{feeds.Fog, fx{clouds: 0.2, overcast: 0.5, fog: true}},
		{feeds.Rain, fx{clouds: 0.8, overcast: 0.7, rain: true}},
		{feeds.Storm, fx{clouds: 1, overcast: 0.9, rain: true, lightning: true}},
		{feeds.Snow, fx{clouds: 0.7, overcast: 0.6, snow: true}},
	} {
		if got := weatherFX(c.k); got != c.want {
			t.Errorf("kind %d: %+v, want %+v", c.k, got, c.want)
		}
	}
	if NewSky(time.Time{}, feeds.View{}).Kind != feeds.Unknown {
		t.Error("no feed should be a clear sky")
	}
	fv := feeds.Demo(at(10, 23, 12, 0))
	if NewSky(at(10, 23, 12, 0), fv).Kind != fv.Weather.Now.Kind {
		t.Error("sky ignores current conditions")
	}
}

// skyFrame renders the dashboard over sky s at animation time T.
func skyFrame(th *ui.Theme, page ui.Page, s Sky, T float64) *Frame {
	snap := sys.Demo(time.Unix(1_800_000_000, 0))
	fv := feeds.Demo(time.Unix(1_800_000_000, 0))
	c := ui.Render(143, 60, ui.View{Page: page, Snap: snap, Feeds: fv, Now: time.Unix(1_800_000_000, 0)}, th, true)
	r := NewRenderer()
	r.Draw(c, th, Anim{T: T, Sky: s})
	return r.Frame()
}

func TestSkyIsDeterministic(t *testing.T) {
	th := ui.Themes[1]
	for _, k := range []feeds.Kind{feeds.Clear, feeds.Rain, feeds.Storm, feeds.Snow, feeds.Fog, feeds.Cloudy} {
		s := Sky{Now: at(10, 23, 17, 20), Kind: k}
		a, b := skyFrame(th, ui.Dash, s, 42.05), skyFrame(th, ui.Dash, s, 42.05)
		for i := range a.Pix {
			if a.Pix[i] != b.Pix[i] {
				t.Fatalf("kind %d: pixel %d differs between renders", k, i)
			}
		}
	}
}

// Day and night must differ; the plain zero sky is the flat backdrop.
func TestSkyChangesWithTime(t *testing.T) {
	th := ui.Themes[1]
	noon := skyFrame(th, ui.Dash, Sky{Now: at(10, 23, 12, 0)}, 0)
	night := skyFrame(th, ui.Dash, Sky{Now: at(10, 23, 2, 0)}, 0)
	plain := skyFrame(th, ui.Dash, Sky{}, 0)
	// The bottom-right corner pixel is backdrop in this layout.
	i := len(noon.Pix) - 1
	if noon.Pix[i] == night.Pix[i] {
		t.Error("noon and night skies are the same")
	}
	if plain.Pix[i] != rgb(th.BG) {
		t.Errorf("plain sky %#06x, want the theme background", plain.Pix[i])
	}
}

// Rain falls with time; with T = 0 (-fps 0) nothing moves.
func TestParticlesMoveOnlyWhenAnimated(t *testing.T) {
	th := ui.Themes[1]
	s := Sky{Now: at(10, 23, 12, 0), Kind: feeds.Rain}
	a, b := skyFrame(th, ui.Dash, s, 10), skyFrame(th, ui.Dash, s, 10.2)
	diff := 0
	for i := range a.Pix {
		if a.Pix[i] != b.Pix[i] {
			diff++
		}
	}
	if diff == 0 {
		t.Error("rain did not move")
	}
	s0, s1 := skyFrame(th, ui.Dash, s, 0), skyFrame(th, ui.Dash, s, 0)
	for i := range s0.Pix {
		if s0.Pix[i] != s1.Pix[i] {
			t.Fatal("still frame not still")
		}
	}
}

// The incremental renderer must equal a full render while the sky turns
// from night to day, the sun moves and the weather changes.
func TestIncrementalSkyMatchesFullRender(t *testing.T) {
	start := time.Unix(1_800_000_000, 0)
	fv := feeds.Demo(start)
	kinds := []feeds.Kind{feeds.Clear, feeds.Storm, feeds.Storm, feeds.Snow, feeds.Fog, feeds.Rain, feeds.Cloudy, feeds.PartlyCloudy, feeds.Unknown}
	for _, th := range []*ui.Theme{ui.Themes[1], ui.Themes[len(ui.Themes)-1]} {
		for _, page := range []ui.Page{ui.Dash, ui.News} {
			inc := NewRenderer()
			for i := range 40 {
				now := start.Add(time.Duration(i) * 211 * time.Second)
				c := ui.Render(107, 45, ui.View{Page: page, Snap: sys.Demo(now), Feeds: fv, Now: now}, th, true)
				wall := at(10, 23, 5, 50).Add(time.Duration(i*i) * 97 * time.Second)
				a := Anim{T: 100 + float64(i)*0.07, Sky: Sky{Now: wall, Kind: kinds[i/5]}}
				if i == 33 {
					a.Sky = Sky{} // back to the plain starfield
				}
				inc.Draw(c, th, a)
				full := NewRenderer()
				full.Draw(c, th, a)
				got, want := inc.Frame().Pix, full.Frame().Pix
				for p := range want {
					if got[p] != want[p] {
						t.Fatalf("%s %v frame %d (%s, kind %d): pixel %d,%d = %#06x, full render %#06x",
							th.Name, page, i, wall.Format("15:04"), a.Sky.Kind, p%inc.Frame().W, p/inc.Frame().W, got[p], want[p])
					}
				}
			}
		}
	}
}

// flashAt is a time when a storm's lightning flashes.
func flashAt() float64 {
	n := uint32(15)
	for hash(n, 99)%3 == 0 {
		n++
	}
	return 7*float64(n) + 0.05 + float64(hash(n, 99)>>4%40)/10
}

// TestSkySamples writes sample frames for eyeballing:
// ORB_SKY_PNGS=dir go test -run SkySamples ./internal/gfx
func TestSkySamples(t *testing.T) {
	dir := os.Getenv("ORB_SKY_PNGS")
	if dir == "" {
		t.Skip("ORB_SKY_PNGS not set")
	}
	times := []struct {
		name string
		t    time.Time
	}{{"night", at(10, 23, 23, 0)}, {"dawn", at(10, 23, 6, 40)}, {"morning", at(10, 23, 9, 30)}, {"noon", at(10, 23, 12, 0)}, {"dusk", at(10, 23, 17, 25)}}
	kinds := []struct {
		name string
		k    feeds.Kind
	}{{"clear", feeds.Clear}, {"partly", feeds.PartlyCloudy}, {"cloudy", feeds.Cloudy}, {"rain", feeds.Rain}, {"storm", feeds.Storm}, {"snow", feeds.Snow}, {"fog", feeds.Fog}}
	for _, th := range ui.Themes {
		for _, tm := range times {
			for _, k := range kinds {
				if k.k != feeds.Clear && tm.name != "noon" && tm.name != "night" {
					continue
				}
				T := 100.0
				if k.k == feeds.Storm {
					T = flashAt()
				}
				f := skyFrame(th, ui.Dash, Sky{Now: tm.t, Kind: k.k}, T)
				name := filepath.Join(dir, fmt.Sprintf("%s-%s-%s.png", th.Name, tm.name, k.name))
				out, err := os.Create(name)
				if err != nil {
					t.Fatal(err)
				}
				if err := WritePNG(out, f, 1); err != nil {
					t.Fatal(err)
				}
				out.Close()
			}
		}
	}
}

// Copying only the returned spans must keep a screen equal to the frame,
// with particles, flashes and the sun moving.
func TestSpansCoverSkyChanges(t *testing.T) {
	th := ui.Themes[0]
	c := ui.Render(107, 45, ui.View{Snap: sys.Demo(time.Unix(1_800_000_000, 0))}, th, true)
	r := NewRenderer()
	var screen []uint32
	T := flashAt() - 1
	for i := range 60 {
		k := []feeds.Kind{feeds.Storm, feeds.Snow, feeds.Fog}[i/20]
		a := Anim{T: T + float64(i)/15, Sky: Sky{Now: at(10, 23, 6, 30).Add(time.Duration(i) * 3 * time.Minute), Kind: k}}
		spans := r.Draw(c, th, a)
		f := r.Frame()
		if screen == nil {
			screen = make([]uint32, len(f.Pix))
		}
		for _, sp := range spans {
			copy(screen[sp.Y*f.W+sp.X0:sp.Y*f.W+sp.X1], f.Pix[sp.Y*f.W+sp.X0:sp.Y*f.W+sp.X1])
		}
		for p := range screen {
			if screen[p] != f.Pix[p] {
				t.Fatalf("frame %d: pixel %d,%d changed outside the spans", i, p%f.W, p/f.W)
			}
		}
	}
}
