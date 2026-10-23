package ui

import (
	"fmt"
	"math"
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
	Play               // calm, between sits: a game of ball (DogAct; never from DogMood)
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
	case Play:
		return "play"
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

// Ball play: a sitting dog plays ball for the last dogPlayLen seconds of
// every dogPlayEvery on the animation clock. Only Sit plays: a napping dog
// means idle and stays asleep, and the working moods have their own gaits.
const (
	dogPlayEvery = 24.0
	dogPlayLen   = 8.0
)

// DogAct is what the dog in mood m does at animation time T (seconds), and
// for Play how far into the bout it is. Sit turns into Play for a bout
// every dogPlayEvery seconds; it still means a moderate load. Play itself
// (for previews) loops the bout.
func DogAct(m Mood, T float64) (Mood, float64) {
	switch m {
	case Sit:
		if u := phase(T, dogPlayEvery) - (dogPlayEvery - dogPlayLen); u >= 0 {
			return Play, u
		}
	case Play:
		return Play, phase(T, dogPlayLen)
	}
	return m, 0
}

// phase is T mod p in [0, p).
func phase(T, p float64) float64 {
	u := math.Mod(T, p)
	if u < 0 {
		u += p
	}
	return u
}

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
