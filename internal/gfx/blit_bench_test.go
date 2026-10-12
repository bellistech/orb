//go:build linux

package gfx

import (
	"fmt"
	"testing"
	"time"

	"nocterm/internal/sys"
	"nocterm/internal/ui"
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
			c := ui.Render(cols, rows, snap, th, true)
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
		c := ui.Render(107, 45, snap, th, true)
		fb.Blit(r.Frame(), r.Draw(c, th, Anim{T: float64(i) / 15}), 4, 8, 0)
	}
}
