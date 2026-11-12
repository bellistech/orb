package gfx

import "time"

// Blank bounds: shorter than BlankMin would fight the boot intro and a
// reader's pauses; BlankMax is only a sanity cap.
const (
	BlankMin = 10 * time.Second
	BlankMax = 24 * time.Hour
)

// wakeGrace is how long after waking a key press is still part of the
// wake-up. The keyboard reaches orb twice, as an input event and as a
// byte on the console; the second must not act (q would quit a dashboard
// someone only meant to wake).
const wakeGrace = 500 * time.Millisecond

// CheckBlank reports whether d is a usable blank time: 0 (never) or
// BlankMin..BlankMax.
func CheckBlank(d time.Duration) bool {
	return d == 0 || d >= BlankMin && d <= BlankMax
}

// Blanker decides when the monitor sleeps: after a set time without
// keyboard or mouse input, until the next input. It is a pure state
// machine fed input times and ticks, like ui.Attract; the caller does
// the blanking.
type Blanker struct {
	after time.Duration // 0: never
	last  time.Time     // last input, or when counting (re)started
	woke  time.Time     // last wake-up
	off   bool
}

// NewBlanker counts idle time from now. after is 0 for never.
func NewBlanker(after time.Duration, now time.Time) *Blanker {
	return &Blanker{after: after, last: now}
}

// Off reports whether the screen is blanked.
func (b *Blanker) Off() bool { return b.off }

// Step is called on every tick while orb owns the screen. It reports
// true once, when the idle time has run out and the screen should go
// dark.
func (b *Blanker) Step(now time.Time) bool {
	if b.after <= 0 || b.off || now.Sub(b.last) < b.after {
		return false
	}
	b.off = true
	return true
}

// Input records keyboard or mouse input at now. It reports whether that
// woke the screen.
func (b *Blanker) Input(now time.Time) bool {
	b.last = now
	if !b.off {
		return false
	}
	b.off, b.woke = false, now
	return true
}

// Waking reports whether a key press at now belongs to the wake-up and
// should do nothing else.
func (b *Blanker) Waking(now time.Time) bool {
	return !b.woke.IsZero() && now.Sub(b.woke) < wakeGrace
}
