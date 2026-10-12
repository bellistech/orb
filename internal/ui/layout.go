package ui

import (
	"nocterm/internal/canvas"
	"nocterm/internal/sys"
)

type drawFn func(canvas.Region, *sys.Snapshot, *Theme)

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
var memGPU = []slot{{drawMem, 42, nil}, {drawGPU, 58, nil}}

// grid picks rows for the terminal width. Weights are proportions.
func grid(w int) []row {
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
func Render(w, h int, s *sys.Snapshot, t *Theme, square bool) *canvas.Canvas {
	c := canvas.New(w, h, t.BG)
	c.Square = square
	full := c.Full()
	if w < 40 || h < 12 {
		full.Text(0, 0, "nocterm needs at least 40x12", t.FG)
		return c
	}
	if s == nil {
		full.Text(2, 1, "sampling…", t.Dim)
		return c
	}
	drawHeader(full.Sub(0, 0, w, headerH), s, t)

	rows := grid(w)
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
			drawSlot(rows[i].slots[j], full.Sub(x, y, ww, hh), s, t)
			x += ww + 1
		}
		y += hh
	}
	return c
}

func drawSlot(sl slot, r canvas.Region, s *sys.Snapshot, t *Theme) {
	if sl.stack == nil {
		sl.draw(r, s, t)
		return
	}
	ws := make([]int, len(sl.stack))
	for i, st := range sl.stack {
		ws[i] = st.weight
	}
	y := 0
	for i, hh := range split(r.H, 0, ws) {
		drawSlot(sl.stack[i], r.Sub(0, y, r.W, hh), s, t)
		y += hh
	}
}
