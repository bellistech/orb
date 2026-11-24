package ui

import (
	"testing"
	"time"

	"orb/internal/feeds"
	"orb/internal/sys"
)

// Key 0 changes the reader's size, which re-paginates every story; the
// reader must stay on the story it was showing, running or paused.
func TestReaderFullKeepsTheStory(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	fv := feeds.Demo(now)
	th := Themes[0]
	for _, paused := range []bool{false, true} {
		for _, step := range []time.Duration{0, 37 * time.Second, 3 * time.Minute, 11*time.Minute + 5*time.Second} {
			var c ReaderClock
			at := now.Add(step)
			render := func(full bool) {
				v := View{Page: News, Snap: sys.Demo(at), Feeds: fv, Now: at, ReaderFull: full}
				c.Apply(&v)
				Render(143, 60, v, th, true)
			}
			render(false)
			if paused {
				c.Toggle(at)
			}
			before := c.Showing(at)
			if !before.ok {
				t.Fatalf("step %v: nothing showing", step)
			}
			render(true) // lay the full size out
			c.Land(before, at)
			if got := c.Showing(at); got != before {
				t.Fatalf("paused=%v step %v: full screen shows %q, was %q", paused, step, got.title, before.title)
			}
			mark := c.Showing(at) // and back, marked before the re-layout as the key does
			render(false)
			c.Land(mark, at)
			if got := c.Showing(at); got != before {
				t.Fatalf("paused=%v step %v: back shows %q, was %q", paused, step, got.title, before.title)
			}
			if c.Paused() != paused {
				t.Fatalf("paused=%v: landing changed the pause", paused)
			}
		}
	}
}
