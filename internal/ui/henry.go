package ui

import (
	"orb/internal/canvas"
)

// Henry Mode: Henry, the dog from the header, out in a meadow under the
// sky. By day he chases butterflies while a turtle walks the stream and
// birds cross overhead; at night he naps, fireflies drift and an owl
// keeps watch. Graphics mode paints it (a canvas scene); a terminal gets
// a few words instead.

// HenryTime is the time of day Henry Mode shows.
type HenryTime uint8

// Times of day, in the order the arrow keys step through them.
const (
	HenryClock HenryTime = iota // follow the clock
	HenryDawn
	HenryDay
	HenryDusk
	HenryNight
	henryTimes
)

var henryTimeNames = [henryTimes]string{"clock", "dawn", "day", "dusk", "night"}

func (h HenryTime) String() string {
	if h >= henryTimes {
		return "clock"
	}
	return henryTimeNames[h]
}

// Step moves to the next (+1) or previous (-1) time of day, wrapping.
func (h HenryTime) Step(d int) HenryTime {
	n := int(henryTimes)
	return HenryTime(((int(h)+d)%n + n) % n)
}

func henryGrid(int) []row {
	return []row{
		{92, []slot{{drawMeadow, 1, nil}}},
		{8, []slot{{drawHenryBar, 1, nil}}},
	}
}

// drawMeadow leaves its cells empty on a graphics canvas and marks them as
// the scene; a terminal gets a framed note.
func drawMeadow(r canvas.Region, v *View, t *Theme) {
	if r.Square() {
		r.AddScene(t.Host)
		return
	}
	in := Frame(r, t, "Henry Mode!", "", t.Host)
	if in.Empty() {
		return
	}
	lines := []struct {
		s string
		c canvas.Color
	}{
		{"Henry is out in the meadow.", t.FG},
		{"", t.FG},
		{"He runs in graphics mode: orb -gfx, then h.", t.Dim},
		{"By day he chases butterflies by the stream; at night he naps", t.Dim},
		{"while the fireflies drift and the owl keeps watch.", t.Dim},
	}
	for i, l := range lines {
		if i >= in.H {
			break
		}
		in.Text(0, i, trunc(l.s, in.W), l.c)
	}
}

// drawHenryBar names the time of day the meadow shows and the keys.
func drawHenryBar(r canvas.Region, v *View, t *Theme) {
	in := Frame(r, t, "Henry Mode!", "← → time of day", t.Host)
	if in.Empty() {
		return
	}
	x := 0
	for h := HenryClock; h < henryTimes; h++ {
		if x > 0 {
			x = in.Text(x, 0, "  ", t.Faint)
		}
		if h == v.HenryTime {
			x = in.Bold(x, 0, h.String(), t.Host)
		} else {
			x = in.Text(x, 0, h.String(), t.Faint)
		}
	}
	night := v.HenryTime == HenryNight
	if v.HenryTime == HenryClock {
		hr := v.Now.Hour()
		night = hr < 6 || hr >= 20
	}
	say := "Henry chases butterflies · the turtle walks the stream · birds overhead"
	if night {
		say = "Henry naps · fireflies drift · the owl keeps watch"
	}
	if in.W-x > 6 {
		in.TextRight(in.W, 0, trunc(say, in.W-x-4), t.Dim)
	}
}
