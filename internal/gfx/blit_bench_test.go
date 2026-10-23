//go:build linux

package gfx

import (
	"fmt"
	"testing"
	"time"

	"orb/internal/canvas"
	"orb/internal/feeds"
	"orb/internal/sys"
	"orb/internal/ui"
)

// Steady-state animated frames on the 3440x1440 desk monitor: render,
// redraw what changed, blit the changed spans. Between samples the canvas is
// reused, as the run loop does.
func BenchmarkAnimatedFrameBlit(b *testing.B) {
	for _, scale := range []int{3, 4, 6} {
		b.Run(fmt.Sprintf("scale%d", scale), func(b *testing.B) {
			pf, _ := newPixelFormat(xrgb())
			fb := &FB{format: pf, W: 3440, H: 1440, stride: 13824}
			fb.mem = make([]byte, fb.stride*fb.H)
			cols, rows, px, py, err := Grid(fb.W, fb.H, scale)
			if err != nil {
				b.Fatal(err)
			}
			snap := sys.Demo(time.Unix(1_800_000_000, 0))
			th := ui.Themes[0]
			c := ui.Render(cols, rows, ui.View{Snap: snap}, th, true)
			r := NewRenderer()
			r.Draw(c, th, Anim{T: 99})
			fb.Blit(r.Frame(), nil, scale, px, py)
			spans := 0
			b.ResetTimer()
			for i := range b.N {
				spans += fb.Blit(r.Frame(), r.Draw(c, th, Anim{T: 100 + float64(i)/15}), scale, px, py)
			}
			b.ReportMetric(float64(spans)/float64(b.N), "spans/frame")
		})
	}
}

// Steady-state animated frames over a weather sky at scale 3: the
// dashboard idle while rain, snow or a storm moves behind it.
func BenchmarkSkyFrameBlit(b *testing.B) {
	for _, k := range []struct {
		name string
		kind feeds.Kind
	}{{"clear", feeds.Clear}, {"cloudy", feeds.Cloudy}, {"rain", feeds.Rain}, {"snow", feeds.Snow}, {"storm", feeds.Storm}, {"fog", feeds.Fog}} {
		b.Run(k.name, func(b *testing.B) {
			pf, _ := newPixelFormat(xrgb())
			fb := &FB{format: pf, W: 3440, H: 1440, stride: 13824}
			fb.mem = make([]byte, fb.stride*fb.H)
			cols, rows, px, py, _ := Grid(fb.W, fb.H, 3)
			snap := sys.Demo(time.Unix(1_800_000_000, 0))
			th := ui.Themes[0]
			c := ui.Render(cols, rows, ui.View{Snap: snap}, th, true)
			sky := Sky{Now: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC), Kind: k.kind}
			r := NewRenderer()
			r.Draw(c, th, Anim{T: 99, Sky: sky})
			fb.Blit(r.Frame(), nil, 3, px, py)
			b.ResetTimer()
			for i := range b.N {
				fb.Blit(r.Frame(), r.Draw(c, th, Anim{T: 100 + float64(i)/15, Sky: sky}), 3, px, py)
			}
		})
	}
}

// Worst case: a tween frame, where every value moved and the dashboard is
// laid out again.
func BenchmarkTweenFrame(b *testing.B) {
	pf, _ := newPixelFormat(xrgb())
	fb := &FB{format: pf, W: 3440, H: 1440, stride: 13824}
	fb.mem = make([]byte, fb.stride*fb.H)
	a, z := sys.Demo(time.Unix(1_800_000_000, 0)), sys.Demo(time.Unix(1_800_000_007, 0))
	th := ui.Themes[0]
	r := NewRenderer()
	for i := range b.N {
		snap := sys.Lerp(a, z, float64(i%10)/10)
		c := ui.Render(107, 45, ui.View{Snap: snap}, th, true)
		fb.Blit(r.Frame(), r.Draw(c, th, Anim{T: float64(i) / 15}), 4, 8, 0)
	}
}

// The theme cycle's cross-fade at 15 fps, frame by frame as the run loop
// draws it: a new palette lays out and redraws the whole frame, a frame
// within a step costs what an idle one does. "dwell" is the same frames
// between fades.
func BenchmarkFadeFrame(b *testing.B) {
	for _, phase := range []struct {
		name string
		at   time.Duration
	}{{"fade", ui.CycleDwell}, {"dwell", 0}} {
		b.Run(phase.name, func(b *testing.B) {
			pf, _ := newPixelFormat(xrgb())
			fb := &FB{format: pf, W: 3440, H: 1440, stride: 13824}
			fb.mem = make([]byte, fb.stride*fb.H)
			cols, rows, px, py, _ := Grid(fb.W, fb.H, 3)
			t0 := time.Unix(1_800_000_000, 0)
			sel, _ := ui.SelectTheme(ui.Cycle, t0)
			snap := sys.Demo(t0)
			r := NewRenderer()
			var c *canvas.Canvas
			var shown *ui.Theme
			frames := int(ui.CycleFade * 15 / time.Second)
			for i := range b.N {
				now := t0.Add(phase.at + time.Duration(i%frames)*time.Second/15)
				th := sel.Theme(now, true)
				if th != shown {
					c, shown = ui.Render(cols, rows, ui.View{Snap: snap, Now: t0}, th, true), th
				}
				fb.Blit(r.Frame(), r.Draw(c, th, Anim{T: 100 + float64(i)/15}), 3, px, py)
			}
		})
	}
}

// The news page lays out every frame while the reader types.
func BenchmarkNewsFrame(b *testing.B) {
	pf, _ := newPixelFormat(xrgb())
	fb := &FB{format: pf, W: 3440, H: 1440, stride: 13824}
	fb.mem = make([]byte, fb.stride*fb.H)
	start := time.Unix(1_800_000_000, 0)
	snap, fv := sys.Demo(start), feeds.Demo(start)
	th := ui.Themes[0]
	r := NewRenderer()
	for i := range b.N {
		now := start.Add(10*time.Second + time.Duration(i)*time.Second/15)
		c := ui.Render(107, 45, ui.View{Page: ui.News, Snap: snap, Feeds: fv, Now: now}, th, true)
		fb.Blit(r.Frame(), r.Draw(c, th, Anim{T: float64(i) / 15}), 4, 8, 0)
	}
}
