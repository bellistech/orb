package ui

import (
	"strings"
	"testing"
	"time"

	"orb/internal/sys"
)

// run steps a from t0 to t0+d once a second, as a run loop's ticks would,
// starting on page p, and returns each page shown and when it came up.
func run(a *Attract, t0 time.Time, d time.Duration, p Page) (pages []Page, at []time.Duration) {
	for el := time.Duration(0); el <= d; el += time.Second {
		if next, ok := a.Step(t0.Add(el), p); ok {
			p = next
			pages, at = append(pages, p), append(at, el)
		}
	}
	return pages, at
}

func TestAttractOffByDefault(t *testing.T) {
	t0 := time.Unix(1_800_000_000, 0)
	a := NewAttract(false)
	if pages, _ := run(a, t0, 10*time.Minute, News); len(pages) != 0 || a.State() != AttractOff {
		t.Fatalf("disarmed attract switched to %v, state %v", pages, a.State())
	}
	var none *Attract
	if none.State() != AttractOff {
		t.Fatal("nil attract is not off")
	}
}

func TestAttractCyclesAfterIdle(t *testing.T) {
	t0 := time.Unix(1_800_000_000, 0)
	a := NewAttract(true) // armed from the start (attract = true)
	a.Step(t0, Dash)
	if a.State() != AttractArmed {
		t.Fatalf("state %v, want armed", a.State())
	}
	pages, at := run(a, t0, 3*time.Minute, Dash)
	// 30 s idle on the dashboard, then on at once; news holds 45 s for a
	// story, net and links 15 s, the dashboard 20 s.
	want := []Page{News, Net, Links, Dash, News}
	wantAt := []time.Duration{30, 75, 90, 105, 125}
	if len(pages) < len(want) {
		t.Fatalf("pages %v at %v", pages, at)
	}
	for i := range want {
		if pages[i] != want[i] || at[i] != wantAt[i]*time.Second {
			t.Fatalf("stop %d: %v at %v, want %v at %ds (all: %v %v)", i, pages[i], at[i], want[i], wantAt[i], pages, at)
		}
	}
	if a.State() != AttractCycling {
		t.Fatalf("state %v, want cycling", a.State())
	}
}

func TestAttractKeyStopsAndRestartsTheWait(t *testing.T) {
	t0 := time.Unix(1_800_000_000, 0)
	a := NewAttract(true)
	a.Step(t0, Dash)
	if p, ok := a.Step(t0.Add(29*time.Second), Dash); ok {
		t.Fatalf("switched to %v before the idle time", p)
	}
	a.Key(t0.Add(29 * time.Second)) // a key just in time
	if _, ok := a.Step(t0.Add(31*time.Second), Dash); ok {
		t.Fatal("cycled 2 s after a key")
	}
	if _, ok := a.Step(t0.Add(59*time.Second), Dash); !ok {
		t.Fatal("no cycle 30 s after the key")
	}
	// A key while cycling stops the cycle where it is.
	a.Key(t0.Add(60 * time.Second))
	if a.State() != AttractArmed {
		t.Fatalf("state %v after a key, want armed", a.State())
	}
	if _, ok := a.Step(t0.Add(80*time.Second), News); ok {
		t.Fatal("cycle went on after a key")
	}
	if p, ok := a.Step(t0.Add(90*time.Second), News); !ok || p != Net {
		t.Fatalf("after 30 s more: %v %v, want net", p, ok)
	}
}

func TestAttractToggle(t *testing.T) {
	t0 := time.Unix(1_800_000_000, 0)
	a := NewAttract(false)
	a.Toggle(t0.Add(time.Hour)) // armed long after start: idle counts from now
	if _, ok := a.Step(t0.Add(time.Hour+29*time.Second), Dash); ok || a.State() != AttractArmed {
		t.Fatal("cycled before the idle time after arming")
	}
	if _, ok := a.Step(t0.Add(time.Hour+30*time.Second), Dash); !ok {
		t.Fatal("no cycle 30 s after arming")
	}
	a.Toggle(t0.Add(time.Hour + 31*time.Second)) // i while cycling disarms
	if pages, _ := run(a, t0.Add(2*time.Hour), 5*time.Minute, Net); len(pages) != 0 || a.State() != AttractOff {
		t.Fatalf("disarmed attract switched to %v", pages)
	}
}

// After a long gap between steps (the console switched away, a long
// sample interval) the cycle moves one page, never several at once.
func TestAttractOneStepAtATime(t *testing.T) {
	t0 := time.Unix(1_800_000_000, 0)
	a := NewAttract(true)
	a.Step(t0, Dash)
	if p, ok := a.Step(t0.Add(10*time.Minute), Dash); !ok || p != News {
		t.Fatalf("%v %v", p, ok)
	}
	if _, ok := a.Step(t0.Add(10*time.Minute+time.Second), News); ok {
		t.Fatal("moved twice")
	}
}

func TestAttractIndicator(t *testing.T) {
	th := Themes[0]
	snap := sys.Demo(time.Unix(1_800_000_000, 0))
	for _, tc := range []struct {
		state AttractState
		want  bool
	}{{AttractOff, false}, {AttractArmed, true}, {AttractCycling, true}} {
		for _, square := range []bool{false, true} {
			c := Render(215, 45, View{Snap: snap, Attract: tc.state}, th, square)
			got := strings.Contains(line(c, 0), "i AUTO")
			if got != tc.want {
				t.Errorf("state %v square %v: indicator %v, want %v: %q", tc.state, square, got, tc.want, line(c, 0))
			}
		}
	}
	// Armed reads like an idle tab, cycling like the active one.
	at := func(s AttractState) bool {
		c := Render(215, 45, View{Snap: snap, Attract: s}, th, true)
		x := strings.Index(line(c, 0), "AUTO")
		return c.CellAt(len([]rune(line(c, 0)[:x])), 0).Bold
	}
	if at(AttractArmed) || !at(AttractCycling) {
		t.Fatal("armed should be faint, cycling lit")
	}
}
