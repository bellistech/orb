package ui

import "time"

// AttractIdle is how long attract mode waits without a key press before it
// starts cycling the pages.
const AttractIdle = 30 * time.Second

// attractDwell is how long the cycle stays on each page: news long enough
// for the reader to tell a whole story (headline, typing, 15 s hold).
func attractDwell(p Page) time.Duration {
	switch p {
	case News:
		return 45 * time.Second
	case Dash:
		return 20 * time.Second
	}
	return 15 * time.Second
}

// AttractState is what the header shows about attract mode.
type AttractState uint8

// Attract mode states.
const (
	AttractOff     AttractState = iota
	AttractArmed                // waiting for the idle time
	AttractCycling              // cycling the pages
)

// Attract is an arcade-style attract mode (Stevie, 2026-10-23: off by
// default, so the news page stays put unless asked). Once armed with i,
// AttractIdle without a key press starts a cycle through the pages, each
// for its dwell time; any key stops it and restarts the wait. Run loops own
// one, report every key to it and Step it on their ticks. It reads no
// clock of its own, so it runs on whatever times it is given.
type Attract struct {
	armed bool
	last  time.Time // last key press, or arming; zero until the first Step
	shown time.Time // when the cycle showed the current page; zero when not cycling
}

// NewAttract returns attract mode, armed or not.
func NewAttract(armed bool) *Attract { return &Attract{armed: armed} }

// Toggle arms or disarms at now (the i key).
func (a *Attract) Toggle(now time.Time) {
	a.armed = !a.armed
	a.last, a.shown = now, time.Time{}
}

// Key records any other key press at now: the cycle stops and the idle
// wait starts over. The caller then handles the key as usual.
func (a *Attract) Key(now time.Time) {
	a.last, a.shown = now, time.Time{}
}

// Step advances to now with page p showing. It returns the page to switch
// to and true when the cycle moves on.
func (a *Attract) Step(now time.Time, p Page) (Page, bool) {
	if !a.armed {
		return p, false
	}
	if a.last.IsZero() {
		a.last = now // armed from the start: idle counts from here
	}
	if a.shown.IsZero() && now.Sub(a.last) < AttractIdle {
		return p, false
	}
	// The page showing when the idle time ran out has had its turn, so
	// the cycle starts by moving on.
	if !a.shown.IsZero() && now.Sub(a.shown) < attractDwell(p) {
		return p, false
	}
	a.shown = now
	return (p + 1) % (Links + 1), true
}

// State reports the attract state for the header.
func (a *Attract) State() AttractState {
	switch {
	case a == nil || !a.armed:
		return AttractOff
	case a.shown.IsZero():
		return AttractArmed
	}
	return AttractCycling
}
