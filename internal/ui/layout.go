package ui

import (
	"time"

	"orb/internal/canvas"
	"orb/internal/feeds"
	"orb/internal/sys"
)

// Page is which screen is showing.
type Page int

// Pages, in key order: d, w, n and l.
const (
	Dash Page = iota
	News
	Net
	Links
)

// View is everything a frame shows.
type View struct {
	Page  Page
	Snap  *sys.Snapshot
	Feeds feeds.View
	Now   time.Time // wall clock for the header and news rotation

	// Reader, when set, positions the news reader; otherwise it runs from
	// the time since the news was fetched.
	Reader ReaderState
}

type drawFn func(canvas.Region, *View, *Theme)

// dash adapts a dashboard panel, which only needs the host snapshot.
func dash(f func(canvas.Region, *sys.Snapshot, *Theme)) drawFn {
	return func(r canvas.Region, v *View, t *Theme) { f(r, v.Snap, t) }
}

// slot is one panel, or (with stack) a column of panels split vertically.
type slot struct {
	draw   drawFn
	weight int
	stack  []slot
}

type row struct {
	weight int
	slots  []slot
}

// headerH is the fixed height of the header band.
const headerH = 5

// memGPU stacks memory over the GPUs: neither fills a tall panel alone.
var memGPU = []slot{{dash(drawMem), 42, nil}, {dash(drawGPU), 58, nil}}

// grid picks rows for the page and width. Weights are proportions.
func grid(p Page, w int) []row {
	switch p {
	case News:
		return newsGrid(w)
	case Net:
		return netGrid(w)
	case Links:
		return linksGrid(w)
	}
	var (
		drawCPU, drawNet, drawDisk = dash(drawCPU), dash(drawNet), dash(drawDisk)
		drawTemp, drawProcs        = dash(drawTemp), dash(drawProcs)
		drawMem, drawGPU           = dash(drawMem), dash(drawGPU)
	)
	switch {
	case w >= 180:
		return []row{
			{55, []slot{{drawCPU, 45, nil}, {nil, 30, memGPU}, {drawTemp, 25, nil}}},
			{45, []slot{{drawNet, 35, nil}, {drawDisk, 30, nil}, {drawProcs, 35, nil}}},
		}
	case w >= 100:
		return []row{
			{40, []slot{{drawCPU, 58, nil}, {nil, 42, memGPU}}},
			{30, []slot{{drawNet, 50, nil}, {drawDisk, 50, nil}}},
			{30, []slot{{drawTemp, 40, nil}, {drawProcs, 60, nil}}},
		}
	default:
		return []row{
			{22, []slot{{drawCPU, 1, nil}}},
			{10, []slot{{drawMem, 1, nil}}},
			{16, []slot{{drawNet, 1, nil}}},
			{14, []slot{{drawDisk, 1, nil}}},
			{14, []slot{{drawTemp, 1, nil}}},
			{12, []slot{{drawGPU, 1, nil}}},
			{12, []slot{{drawProcs, 1, nil}}},
		}
	}
}

// split divides n cells by weights, leaving gap cells between parts.
func split(n, gap int, weights []int) []int {
	total := 0
	for _, w := range weights {
		total += w
	}
	avail := n - gap*(len(weights)-1)
	out := make([]int, len(weights))
	used := 0
	for i, w := range weights {
		if i == len(weights)-1 {
			out[i] = avail - used
			break
		}
		out[i] = avail * w / max(total, 1)
		used += out[i]
	}
	return out
}

// Render draws one frame of w x h cells. square is true for graphics tiles.
func Render(w, h int, v View, t *Theme, square bool) *canvas.Canvas {
	c := canvas.New(w, h, t.BG)
	c.Square = square
	full := c.Full()
	if w < 40 || h < 12 {
		full.Text(0, 0, "orb needs at least 40x12", t.FG)
		return c
	}
	if v.Snap == nil {
		full.Text(2, 1, "sampling…", t.Dim)
		return c
	}
	if v.Now.IsZero() {
		v.Now = v.Snap.Time
	}
	drawHeader(full.Sub(0, 0, w, headerH), &v, t)

	rows := grid(v.Page, w)
	weights := make([]int, len(rows))
	for i, r := range rows {
		weights[i] = r.weight
	}
	y := headerH
	for i, hh := range split(h-headerH, 0, weights) {
		ws := make([]int, len(rows[i].slots))
		for j, sl := range rows[i].slots {
			ws[j] = sl.weight
		}
		x := 0
		for j, ww := range split(w, 1, ws) {
			drawSlot(rows[i].slots[j], full.Sub(x, y, ww, hh), &v, t)
			x += ww + 1
		}
		y += hh
	}
	return c
}

func drawSlot(sl slot, r canvas.Region, v *View, t *Theme) {
	if sl.stack == nil {
		sl.draw(r, v, t)
		return
	}
	ws := make([]int, len(sl.stack))
	for i, st := range sl.stack {
		ws[i] = st.weight
	}
	y := 0
	for i, hh := range split(r.H, 0, ws) {
		drawSlot(sl.stack[i], r.Sub(0, y, r.W, hh), v, t)
		y += hh
	}
}
