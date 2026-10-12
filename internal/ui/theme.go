// Package ui draws the dashboard onto a canvas.
package ui

import (
	"github.com/lucasb-eyer/go-colorful"

	"nocterm/internal/canvas"
)

// Theme is a palette. Ramp runs cool -> hot and colors every value.
type Theme struct {
	Name  string
	BG    canvas.Color // gutters and outside panels
	Panel canvas.Color // panel interiors
	Track canvas.Color // empty part of a gauge
	FG    canvas.Color
	Dim   canvas.Color
	Faint canvas.Color
	RX    canvas.Color
	TX    canvas.Color

	Host, CPU, Mem, GPU, Net, Disk, Temp, Proc canvas.Color

	// Graphics-mode windows; unset means derived (see Window).
	WinTop, WinBottom, Edge canvas.Color

	ramp []canvas.Color // pre-blended, rampSteps entries
}

const rampSteps = 128

func h(s string) canvas.Color { return canvas.Hex(s) }

// Themes in cycle order; the first is the default.
var Themes = []*Theme{
	newTheme(Theme{
		Name: "phosphor",
		BG:   h("#0a0702"), Panel: h("#100b03"), Track: h("#2a1f09"),
		FG: h("#ffcc66"), Dim: h("#b38a3d"), Faint: h("#5a4318"),
		RX: h("#ffd080"), TX: h("#ff9a3d"),
		Host: h("#ffd27f"), CPU: h("#ffb000"), Mem: h("#ffc040"), GPU: h("#ffa040"),
		Net: h("#ffd080"), Disk: h("#e6a020"), Temp: h("#ff8040"), Proc: h("#ffcc66"),
	}, "#7a5a1a", "#c98f1e", "#ffb000", "#ff7a1a", "#ff3d1a"),
	newTheme(Theme{
		Name: "gizzard",
		BG:   h("#0b0812"), Panel: h("#110c1b"), Track: h("#221a33"),
		FG: h("#efe8fb"), Dim: h("#a197bb"), Faint: h("#4d4366"),
		RX: h("#3dffa2"), TX: h("#ff5fd2"),
		Host: h("#c58bff"), CPU: h("#19d3ff"), Mem: h("#ff5fd2"), GPU: h("#a98bff"),
		Net: h("#3dffa2"), Disk: h("#ffb347"), Temp: h("#ff6b6b"), Proc: h("#6fb6ff"),
	}, "#19d3ff", "#3dffa2", "#c6ff3d", "#ffd23d", "#ff8a2a", "#ff2e88"),
	newTheme(Theme{
		Name: "lantern",
		BG:   h("#181825"), Panel: h("#1e1e2e"), Track: h("#313244"),
		FG: h("#cdd6f4"), Dim: h("#a6adc8"), Faint: h("#585b70"),
		RX: h("#a6e3a1"), TX: h("#f5c2e7"),
		Host: h("#cba6f7"), CPU: h("#89b4fa"), Mem: h("#f5c2e7"), GPU: h("#b4befe"),
		Net: h("#a6e3a1"), Disk: h("#f9e2af"), Temp: h("#f38ba8"), Proc: h("#74c7ec"),
	}, "#89dceb", "#a6e3a1", "#f9e2af", "#fab387", "#f38ba8"),
	newTheme(Theme{
		Name: "chrono",
		BG:   h("#04051a"), Panel: h("#101b6e"), Track: h("#0a0f45"),
		FG: h("#ffffff"), Dim: h("#b8c4ff"), Faint: h("#6a78c8"),
		RX: h("#7dffb0"), TX: h("#ff8fd8"),
		Host: h("#ffe066"), CPU: h("#5ce1ff"), Mem: h("#ff8fd8"), GPU: h("#c3a6ff"),
		Net: h("#7dffb0"), Disk: h("#ffc04d"), Temp: h("#ff6b6b"), Proc: h("#8fc8ff"),
		WinTop: h("#3b4fc4"), WinBottom: h("#0b1150"), Edge: h("#eef0ff"),
	}, "#5ce1ff", "#7dffb0", "#ffe066", "#ff9a3d", "#ff4d6d"),
}

// ThemeByName returns the named theme, or nil.
func ThemeByName(name string) *Theme {
	for _, t := range Themes {
		if t.Name == name {
			return t
		}
	}
	return nil
}

func newTheme(t Theme, stops ...string) *Theme {
	cs := make([]colorful.Color, len(stops))
	for i, s := range stops {
		cs[i], _ = colorful.Hex(s)
	}
	t.ramp = make([]canvas.Color, rampSteps)
	for i := range t.ramp {
		p := float64(i) / float64(rampSteps-1) * float64(len(cs)-1)
		seg := min(int(p), len(cs)-2)
		c := cs[seg].BlendOkLab(cs[seg+1], p-float64(seg)).Clamped()
		r, g, b := c.RGB255()
		t.ramp[i] = canvas.RGB(r, g, b)
	}
	return &t
}

// Heat maps p in [0,1] onto the cool -> hot ramp.
func (t *Theme) Heat(p float64) canvas.Color {
	p = max(0, min(1, p))
	return t.ramp[int(p*float64(rampSteps-1)+0.5)]
}

// Mix blends a toward b by t in OKLab.
func Mix(a, b canvas.Color, t float64) canvas.Color {
	ca := colorful.Color{R: float64(a.R) / 255, G: float64(a.G) / 255, B: float64(a.B) / 255}
	cb := colorful.Color{R: float64(b.R) / 255, G: float64(b.G) / 255, B: float64(b.B) / 255}
	r, g, bl := ca.BlendOkLab(cb, t).Clamped().RGB255()
	return canvas.RGB(r, g, bl)
}

// Window returns the gradient top, gradient bottom and bevel colors for a
// graphics-mode window with the given accent. Themes may set them
// explicitly; otherwise they are derived from the panel color and accent.
func (t *Theme) Window(accent canvas.Color) (top, bottom, edge canvas.Color) {
	top, bottom, edge = t.WinTop, t.WinBottom, t.Edge
	if !top.Set {
		top = Mix(t.Panel, accent, 0.16)
	}
	if !bottom.Set {
		bottom = Mix(t.Panel, t.BG, 0.6)
	}
	if !edge.Set {
		edge = Mix(t.FG, accent, 0.35)
	}
	return top, bottom, edge
}
