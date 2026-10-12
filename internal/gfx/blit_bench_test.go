//go:build linux

package gfx

import (
	"testing"
	"time"

	"nocterm/internal/sys"
	"nocterm/internal/ui"
)

// A steady-state animated frame on the desk monitor: render, rasterize and
// blit only the rows that changed since the previous frame.
func BenchmarkAnimatedFrameBlit(b *testing.B) {
	pf, _ := newPixelFormat(xrgb())
	fb := &FB{format: pf, W: 3440, H: 1440, stride: 13824}
	fb.mem = make([]byte, fb.stride*fb.H)
	snap := sys.Demo(time.Unix(1_800_000_000, 0))
	th := ui.Themes[0]
	cur, prev := NewFrame(107*Tile, 45*Tile), NewFrame(107*Tile, 45*Tile)
	rows := 0
	for i := range b.N {
		Rasterize(cur, ui.Render(107, 45, snap, th, true), th, Anim{T: 100 + float64(i)/15})
		rows += fb.Blit(cur, prev, 4, 8, 0)
		cur, prev = prev, cur
	}
	b.ReportMetric(float64(rows)/float64(b.N), "rows/frame")
}
