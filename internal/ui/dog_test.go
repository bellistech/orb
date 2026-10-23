package ui

import (
	"testing"
	"time"

	"orb/internal/sys"
)

// calm is a quiet host: idle CPU, half the memory, cool sensors, roomy disks.
func calm(cpu ...float64) *sys.Snapshot {
	s := &sys.Snapshot{
		Mem:    sys.Mem{Total: 100, Used: 50, SwapTotal: 10},
		Temps:  []sys.Temp{{Chip: "cpu", Celsius: 45, Crit: 95}, {Chip: "nvme", Celsius: 40}},
		Mounts: []sys.Mount{{Path: "/", Used: 50, Avail: 50}},
	}
	s.CPUHist = cpu
	if len(cpu) > 0 {
		s.CPU = cpu[len(cpu)-1]
	}
	return s
}

func TestDogMood(t *testing.T) {
	cases := []struct {
		name string
		snap func() *sys.Snapshot
		mood Mood
		say  string
	}{
		{"no snapshot", func() *sys.Snapshot { return nil }, Nap, ""},
		{"empty", func() *sys.Snapshot { return &sys.Snapshot{} }, Nap, ""},
		{"idle", func() *sys.Snapshot { return calm(0.02, 0.05, 0.03) }, Nap, ""},
		{"just under sit", func() *sys.Snapshot { return calm(0.14, 0.14, 0.14) }, Nap, ""},
		{"moderate", func() *sys.Snapshot { return calm(0.3, 0.3, 0.3) }, Sit, ""},
		{"no history uses CPU", func() *sys.Snapshot { s := calm(); s.CPU = 0.3; return s }, Sit, ""},
		{"busy", func() *sys.Snapshot { return calm(0.6, 0.65, 0.7) }, Trot, ""},
		{"pegged", func() *sys.Snapshot { return calm(0.95, 0.99, 1) }, Sprint, ""},
		{"one spike does not sprint", func() *sys.Snapshot { return calm(0.02, 0.02, 0.02, 1) }, Sit, ""},
		{"old load is forgotten", func() *sys.Snapshot { return calm(1, 1, 1, 0.02, 0.02, 0.02) }, Nap, ""},
		{"memory pressure", func() *sys.Snapshot { s := calm(0.02); s.Mem.Used = 92; return s }, Pant, ""},
		{"swapping", func() *sys.Snapshot { s := calm(0.3); s.Mem.SwapUsed = 6; return s }, Pant, ""},
		{"hot sensor", func() *sys.Snapshot { s := calm(0.3); s.Temps[0].Celsius = 88; return s }, Pant, ""},
		{"warm is fine", func() *sys.Snapshot { s := calm(0.3); s.Temps[0].Celsius = 75; return s }, Sit, ""},
		{"pegged and hot sprints", func() *sys.Snapshot { s := calm(1, 1, 1); s.Temps[0].Celsius = 90; return s }, Sprint, ""},
		{"disk almost full", func() *sys.Snapshot {
			s := calm(0.02)
			s.Mounts = append(s.Mounts, sys.Mount{Path: "/srv", Used: 97, Avail: 3})
			return s
		}, Bark, "disk 97%"},
		{"disk at 94% is fine", func() *sys.Snapshot { s := calm(0.3); s.Mounts[0] = sys.Mount{Used: 94, Avail: 6}; return s }, Sit, ""},
		{"past critical", func() *sys.Snapshot { s := calm(0.3); s.Temps[0].Celsius = 96; return s }, Bark, "cpu 96°"},
		{"no crit reported: 100°", func() *sys.Snapshot { s := calm(0.3); s.Temps[1].Celsius = 101; return s }, Bark, "nvme 101°"},
		{"bogus crit ignored", func() *sys.Snapshot {
			s := calm(0.3)
			s.Temps[0] = sys.Temp{Chip: "cpu", Celsius: 50, Crit: 20}
			return s
		}, Sit, ""},
		{"alert beats a pegged CPU", func() *sys.Snapshot { s := calm(1, 1, 1); s.Mounts[0] = sys.Mount{Used: 99, Avail: 1}; return s }, Bark, "disk 99%"},
	}
	for _, c := range cases {
		m, say := DogMood(c.snap())
		if m != c.mood || say != c.say {
			t.Errorf("%s: got %v %q, want %v %q", c.name, m, say, c.mood, c.say)
		}
	}
}

func TestMoodString(t *testing.T) {
	for m, want := range []string{"nap", "sit", "trot", "sprint", "pant", "bark"} {
		if got := Mood(m).String(); got != want {
			t.Errorf("Mood(%d) = %q, want %q", m, got, want)
		}
	}
	if Mood(99).String() != "?" {
		t.Error("unknown mood should print ?")
	}
}

// The pen sits in free header cells only (graphics canvases at desk
// width), carries the mood, and never covers text.
func TestDogPen(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	demo := sys.Demo(now)
	th := Themes[1]
	c := Render(143, 60, View{Snap: demo, Now: now}, th, true)
	if len(c.Pets) != 1 {
		t.Fatalf("want one pen at desk width, got %d", len(c.Pets))
	}
	p := c.Pets[0]
	if p.Y != 0 || p.H != 3 || p.W < 12 || p.X < 40 {
		t.Fatalf("pen %+v not in the header's free space", p)
	}
	if m, say := DogMood(demo); Mood(p.Mood) != m || p.Say != say {
		t.Fatalf("pen mood %v %q, want %v %q", Mood(p.Mood), p.Say, m, say)
	}
	for y := 1; y < p.Y+p.H; y++ {
		for x := p.X; x < p.X+p.W; x++ {
			if ch := c.CellAt(x, y).Ch; ch != ' ' {
				t.Fatalf("pen covers %q at %d,%d", ch, x, y)
			}
		}
	}
	if c := Render(143, 45, View{Snap: demo, Now: now}, th, false); len(c.Pets) != 0 {
		t.Fatal("terminal canvas got a pen")
	}
	if c := Render(80, 40, View{Snap: demo, Now: now}, th, true); len(c.Pets) != 0 {
		t.Fatal("narrow canvas got a pen")
	}
}
