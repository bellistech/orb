package ui

import (
	"nocterm/internal/canvas"
	"nocterm/internal/sys"
)

type drawFn func(canvas.Region, *sys.Snapshot, *Theme)

type slot struct {
	draw   drawFn
	weight int
}

type row struct {
	weight int
	slots  []slot
}

// headerH is the fixed height of the header band.
const headerH = 5

// grid picks rows for the terminal width. Weights are proportions.
func grid(w int) []row {
	switch {
	case w >= 180:
		return []row{
			{55, []slot{{drawCPU, 46}, {drawMem, 27}, {drawGPU, 27}}},
			{45, []slot{{drawNet, 26}, {drawDisk, 26}, {drawTemp, 22}, {drawProcs, 26}}},
		}
	case w >= 100:
		return []row{
			{34, []slot{{drawCPU, 60}, {drawMem, 40}}},
			{30, []slot{{drawNet, 50}, {drawGPU, 50}}},
			{36, []slot{{drawDisk, 34}, {drawTemp, 30}, {drawProcs, 36}}},
		}
	default:
		return []row{
			{22, []slot{{drawCPU, 1}}},
			{12, []slot{{drawMem, 1}}},
			{16, []slot{{drawNet, 1}}},
			{14, []slot{{drawDisk, 1}}},
			{12, []slot{{drawTemp, 1}}},
			{12, []slot{{drawGPU, 1}}},
			{12, []slot{{drawProcs, 1}}},
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

// Render draws one frame of size w x h.
func Render(w, h int, s *sys.Snapshot, t *Theme) *canvas.Canvas {
	c := canvas.New(w, h, t.BG)
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
			rows[i].slots[j].draw(full.Sub(x, y, ww, hh), s, t)
			x += ww + 1
		}
		y += hh
	}
	return c
}
