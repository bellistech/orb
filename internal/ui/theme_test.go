package ui

import (
	"math"
	"testing"

	"orb/internal/canvas"
)

// wcag is the WCAG 2 contrast ratio of two colors.
func wcag(x, y canvas.Color) float64 {
	lum := func(c canvas.Color) float64 {
		f := func(v uint8) float64 {
			s := float64(v) / 255
			if s <= 0.04045 {
				return s / 12.92
			}
			return math.Pow((s+0.055)/1.055, 2.4)
		}
		return 0.2126*f(c.R) + 0.7152*f(c.G) + 0.0722*f(c.B)
	}
	lx, ly := lum(x), lum(y)
	return (max(lx, ly) + 0.05) / (min(lx, ly) + 0.05)
}

func TestSylvanRegistered(t *testing.T) {
	if th := ThemeByName("sylvan"); th == nil || th.Heat(0) == th.Heat(1) {
		t.Fatal("sylvan missing or flat")
	}
}

// Text on panels and window gradients must stay readable: WCAG AA for
// body text (4.5:1) for FG, AA large text (3:1) for Dim and the accents
// (titles, bold values).
func TestThemeContrast(t *testing.T) {
	for _, th := range Themes {
		top, bot, _ := th.Window(th.CPU)
		worst := func(c canvas.Color) float64 { return min(wcag(c, th.Panel), wcag(c, top), wcag(c, bot)) }
		if r := worst(th.FG); r < 4.5 {
			t.Errorf("%s: FG contrast %.1f", th.Name, r)
		}
		if r := worst(th.Dim); r < 3 {
			t.Errorf("%s: Dim contrast %.1f", th.Name, r)
		}
		if th.Name != "sylvan" {
			continue // older themes keep their looks; chrono's are lower
		}
		for _, a := range []canvas.Color{th.Host, th.CPU, th.Mem, th.GPU, th.Net, th.Disk, th.Temp, th.Proc, th.RX, th.TX} {
			if r := worst(a); r < 3 {
				t.Errorf("%s: accent %v contrast %.1f", th.Name, a, r)
			}
		}
	}
}
