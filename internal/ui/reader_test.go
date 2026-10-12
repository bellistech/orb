package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"nocterm/internal/feeds"
)

func TestStoriesShuffledAndSkipEmpty(t *testing.T) {
	var items []feeds.Item
	for i := range 30 {
		items = append(items, feeds.Item{Title: fmt.Sprint(i), Summary: "x"})
	}
	n := &feeds.News{Categories: []feeds.Category{
		{Name: "A", Items: items},
		{Name: "B", Items: []feeds.Item{{Title: "no summary"}}},
	}}
	order := func(seed uint64) string {
		var got []string
		for _, s := range stories(n, seed) {
			got = append(got, s.item.Title)
		}
		return strings.Join(got, ",")
	}
	a, b := order(1), order(2)
	if a != order(1) {
		t.Fatal("same seed gave a different order")
	}
	if a == b {
		t.Fatal("different seeds gave the same order")
	}
	if strings.Contains(a, "no summary") || len(strings.Split(a, ",")) != 30 {
		t.Fatalf("wrong set: %s", a)
	}
	var natural []string
	for i := range 30 {
		natural = append(natural, fmt.Sprint(i))
	}
	if a == strings.Join(natural, ",") {
		t.Fatal("order not shuffled")
	}
	if stories(nil, 1) != nil {
		t.Fatal("nil news")
	}
}

func TestLocateWalksTheSchedule(t *testing.T) {
	ss := []story{
		{0, feeds.Item{Title: "one", Summary: "aaaa bbbb cccc dddd"}}, // 4 lines of 4 at w=4
		{1, feeds.Item{Title: "two", Summary: "ee"}},
	}
	plans, total := planReader(ss, 4, 2) // story one: 2 pages of 2 lines
	if len(plans) != 2 || len(plans[0].pages) != 2 || plans[0].pages[0].chars != 8 {
		t.Fatalf("plans = %+v", plans)
	}
	sec := func(f float64) time.Duration { return time.Duration(f * float64(time.Second)) }
	for _, tc := range []struct {
		at    time.Duration
		want  readerPos
		label string
	}{
		{0, readerPos{0, 0, 0, false}, "intro shows no body"},
		{readerIntro + sec(0.1), readerPos{0, 0, 4, false}, "typing at 40/s"},
		{readerIntro + sec(0.5), readerPos{0, 0, 8, true}, "page typed, holding"},
		{readerIntro + sec(0.2) + readerHold + sec(0.1), readerPos{0, 1, 4, false}, "second page types"},
		{total + readerIntro + sec(0.1), readerPos{0, 0, 4, false}, "cycle wraps"},
		{-sec(1), readerPos{1, 0, 2, true}, "negative elapsed wraps"},
	} {
		if got := locate(plans, total, tc.at); got != tc.want {
			t.Errorf("%s: locate(%v) = %+v, want %+v", tc.label, tc.at, got, tc.want)
		}
	}
	if (locate(nil, 0, time.Second) != readerPos{}) {
		t.Fatal("empty plan")
	}
}

func TestWrapAll(t *testing.T) {
	got := wrapAll("aa bbbbbbb c", 4)
	if strings.Join(got, "|") != "aa|bbbb|bbb|c" {
		t.Fatalf("got %q", got)
	}
	if wrapAll("x", 0) != nil {
		t.Fatal("zero width")
	}
}

func TestReaderClock(t *testing.T) {
	t0 := time.Unix(1_800_000_000, 0)
	n1, n2 := &feeds.News{}, &feeds.News{}
	view := func(n *feeds.News, at, now time.Time) View {
		return View{Now: now, Feeds: feeds.View{News: n, NewsAt: at}}
	}
	var c ReaderClock
	v := view(n1, t0, t0.Add(10*time.Second))
	c.Apply(&v)
	if v.Reader.Elapsed != 10*time.Second || v.Reader.Paused {
		t.Fatalf("running: %+v", v.Reader)
	}

	c.Toggle(t0.Add(10 * time.Second)) // pause
	// Time passes and a new fetch lands: the reader stays put on the old news.
	v = view(n2, t0.Add(60*time.Second), t0.Add(70*time.Second))
	c.Apply(&v)
	if !v.Reader.Paused || v.Reader.Elapsed != 10*time.Second || v.Feeds.News != n1 || !v.Feeds.NewsAt.Equal(t0) {
		t.Fatalf("paused: %+v news=%p", v.Reader, v.Feeds.News)
	}

	c.Toggle(t0.Add(70 * time.Second)) // resume: 60s were spent paused
	v = view(n1, t0, t0.Add(75*time.Second))
	c.Apply(&v)
	if v.Reader.Paused || v.Reader.Elapsed != 15*time.Second {
		t.Fatalf("resumed: %+v", v.Reader)
	}

	// New news after resuming restarts the cycle.
	v = view(n2, t0.Add(80*time.Second), t0.Add(83*time.Second))
	c.Apply(&v)
	if v.Reader.Elapsed != 3*time.Second {
		t.Fatalf("new news: %+v", v.Reader)
	}
}
