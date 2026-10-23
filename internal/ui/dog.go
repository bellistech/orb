package ui

import (
	"fmt"
	"strings"

	"orb/internal/canvas"
	"orb/internal/sys"
)

// Mood is what the dog in the header is doing. It is a status signal: the
// busier or hotter the host, the livelier the dog.
type Mood uint8

// Moods, calmest first.
const (
	Nap    Mood = iota // CPU idle
	Sit                // moderate: sits, wags, looks around
	Trot               // busy: trots back and forth
	Sprint             // pegged: sprints
	Pant               // memory pressure or a hot sensor (short of pegged)
	Bark               // alert: a disk almost full, a sensor past critical
)

func (m Mood) String() string {
	switch m {
	case Nap:
		return "nap"
	case Sit:
		return "sit"
	case Trot:
		return "trot"
	case Sprint:
		return "sprint"
	case Pant:
		return "pant"
	case Bark:
		return "bark"
	}
	return "?"
}

// Thresholds for the dog's mood. CPU is the mean of the last dogWindow
// samples, so one spike does not wake it.
const (
	dogWindow    = 3
	dogSit       = 0.15 // below: nap
	dogTrot      = 0.45
	dogSprint    = 0.85
	dogMemPant   = 0.90 // memory used
	dogSwapPant  = 0.50 // swap used
	dogHotPant   = 0.85 // tempFrac of the hottest sensor
	dogDiskAlert = 0.95
)

// DogMood maps a snapshot to the dog's mood, and for Bark what it says
// ("disk 97%", "cpu 96°"). Alerts come first, then a pegged CPU, then heat
// and memory pressure, then the CPU level.
func DogMood(s *sys.Snapshot) (Mood, string) {
	if s == nil {
		return Nap, ""
	}
	if m := fullestMount(s.Mounts); m != nil && m.UsedFrac() >= dogDiskAlert {
		return Bark, "disk " + strings.TrimSpace(Pct(m.UsedFrac()))
	}
	for _, tp := range s.Temps {
		if critical(tp) {
			return Bark, fmt.Sprintf("%s %.0f°", tp.Chip, tp.Celsius)
		}
	}
	cpu := s.CPU
	if len(s.CPUHist) > 0 {
		cpu = mean(tail(s.CPUHist, dogWindow))
	}
	// A pegged CPU runs hot by design (boost to its limit), so sprinting
	// beats panting; heat or memory pressure below that pants.
	if cpu >= dogSprint {
		return Sprint, ""
	}
	swap := frac(s.Mem.SwapUsed, s.Mem.SwapTotal)
	if s.Mem.UsedFrac() >= dogMemPant || swap >= dogSwapPant {
		return Pant, ""
	}
	if hot := hottest(s.Temps); hot != nil && tempFrac(*hot) >= dogHotPant {
		return Pant, ""
	}
	switch {
	case cpu >= dogTrot:
		return Trot, ""
	case cpu >= dogSit:
		return Sit, ""
	}
	return Nap, ""
}

// critical reports a sensor at or past its critical point (100°C when it
// reports none).
func critical(tp sys.Temp) bool {
	crit := 100.0
	if tp.Crit > 60 && tp.Crit < 130 {
		crit = tp.Crit
	}
	return tp.Celsius >= crit
}

// dogPen records the dog's patch of the header: the free cells between
// the identity lines and the date, on the top two rows, reaching up into
// the border so the dog stands on the gap above the third row. Only on
// square (graphics) canvases, and only when there is room to walk.
func dogPen(r canvas.Region, x0, x1 int, s *sys.Snapshot, t *Theme) {
	const minW = 12
	if !r.Square() || x1-x0 < minW || r.H < 3 {
		return
	}
	m, say := DogMood(s)
	r.Sub(x0, 0, x1-x0, 3).AddPet(t.Host, uint8(m), say)
}
