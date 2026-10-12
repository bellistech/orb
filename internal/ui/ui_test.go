package ui

import (
	"strings"
	"testing"
	"time"

	"orb/internal/canvas"
	"orb/internal/feeds"
	"orb/internal/sys"
)

func line(c *canvas.Canvas, y int) string { return strings.Split(c.Plain(), "\n")[y] }

func TestGauge(t *testing.T) {
	th := Themes[0]
	for _, tc := range []struct {
		f    float64
		w    int
		want string
	}{
		{0, 4, "    "},
		{0.5, 4, "██  "},
		{1, 4, "████"},
		{0.0625, 2, "▏ "},
		{0.25, 1, "▎"},
		{2, 3, "███"}, // clamped
	} {
		c := canvas.New(tc.w, 1, th.BG)
		Gauge(c.Full(), 0, 0, tc.w, tc.f, th, th.Heat)
		if got := line(c, 0); got != tc.want {
			t.Errorf("Gauge(%v, w=%d) = %q, want %q", tc.f, tc.w, got, tc.want)
		}
	}
}

func TestGraphFullAndDown(t *testing.T) {
	th := Themes[0]
	c := canvas.New(2, 2, th.BG)
	Graph(c.Full(), []float64{1, 1, 1, 1}, false, th, th.Heat)
	if got := c.Plain(); got != "⣿⣿\n⣿⣿" {
		t.Fatalf("full graph = %q", got)
	}
	// Half height grows from the base: bottom row full, top empty.
	c = canvas.New(1, 2, th.BG)
	Graph(c.Full(), []float64{0.5, 0.5}, false, th, th.Heat)
	if got := c.Plain(); got != " \n⣿" {
		t.Fatalf("half graph = %q", got)
	}
	c = canvas.New(1, 2, th.BG)
	Graph(c.Full(), []float64{0.5, 0.5}, true, th, th.Heat)
	if got := c.Plain(); got != "⣿\n " {
		t.Fatalf("down graph = %q", got)
	}
}

func TestSplit(t *testing.T) {
	got := split(100, 1, []int{46, 27, 27})
	sum := 0
	for _, v := range got {
		sum += v
	}
	if sum != 98 {
		t.Fatalf("split = %v, sum %d, want 98", got, sum)
	}
}

func TestHeatEndpoints(t *testing.T) {
	for _, th := range Themes {
		if th.Heat(0) == th.Heat(1) {
			t.Errorf("%s: ramp is flat", th.Name)
		}
		if th.Heat(-1) != th.Heat(0) || th.Heat(2) != th.Heat(1) {
			t.Errorf("%s: Heat does not clamp", th.Name)
		}
	}
}

// Every size, theme and data source must render without panicking and fill
// exactly w x h cells.
func TestRenderAnySize(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	demo := sys.Demo(now)
	empty := &sys.Snapshot{Time: demo.Time}
	views := []View{
		{Page: Dash, Snap: demo}, {Page: Dash, Snap: empty}, {Page: Dash},
		{Page: News, Snap: demo, Feeds: feeds.Demo(now), Now: now},
		{Page: News, Snap: demo, Feeds: feeds.View{NewsOn: true, WeatherErr: "timeout", WeatherOn: true}},
		{Page: News, Snap: empty},
		{Page: Net, Snap: demo}, {Page: Net, Snap: empty},
	}
	for _, th := range Themes {
		for _, v := range views {
			for w := 1; w <= 260; w += 7 {
				for h := 1; h <= 90; h += 5 {
					lines := strings.Split(Render(w, h, v, th, w%2 == 0).Plain(), "\n")
					if len(lines) != h {
						t.Fatalf("%s %dx%d: %d lines", th.Name, w, h, len(lines))
					}
					for i, l := range lines {
						if n := len([]rune(l)); n != w {
							t.Fatalf("%s %dx%d: line %d has %d cells", th.Name, w, h, i, n)
						}
					}
				}
			}
		}
	}
}

func TestBytes(t *testing.T) {
	for in, want := range map[float64]string{0: "0B", 1023: "1023B", 1536: "1.5K", 12 << 30: "12G"} {
		if got := Bytes(in); got != want {
			t.Errorf("Bytes(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestWrap(t *testing.T) {
	for _, tc := range []struct {
		s    string
		w, n int
		want string
	}{
		{"one two three", 7, 2, "one two|three"},
		{"one two three four", 7, 2, "one two|three …"},
		{"supercalifragilistic", 5, 2, "supe…"},
		{"", 5, 2, ""},
		{"a b", 0, 2, ""},
	} {
		if got := strings.Join(wrap(tc.s, tc.w, tc.n), "|"); got != tc.want {
			t.Errorf("wrap(%q,%d,%d) = %q, want %q", tc.s, tc.w, tc.n, got, tc.want)
		}
	}
}

func TestHourLabel(t *testing.T) {
	for _, tc := range []struct {
		in   string
		w    int
		want string
	}{{"11 AM", 5, "11AM"}, {"11 AM", 3, "11"}, {"9 PM", 4, "9PM"}, {"12 PM", 1, "…"}} {
		if got := hourLabel(tc.in, tc.w); got != tc.want {
			t.Errorf("hourLabel(%q,%d) = %q, want %q", tc.in, tc.w, got, tc.want)
		}
	}
}
