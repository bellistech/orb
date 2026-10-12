package ui

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"nocterm/internal/canvas"
	"nocterm/internal/feeds"
)

// Reader pacing: the headline shows alone briefly, then the summary types
// out at roughly reading speed, page by page, holding each full page.
const (
	typeRate    = 40 // characters per second
	readerIntro = 1500 * time.Millisecond
	readerHold  = 5 * time.Second // pause once a page is fully typed
	maxStories  = 48
	readerWidth = 72 // comfortable line length for reading
)

type story struct {
	cat  int
	item feeds.Item
}

// stories returns every item that has a summary, in a random order seeded
// by seed (the fetch time), so the order is stable between frames and
// reshuffles when new news arrives.
func stories(n *feeds.News, seed uint64) []story {
	if n == nil {
		return nil
	}
	var out []story
	for ci, c := range n.Categories {
		for _, it := range c.Items {
			if it.Summary != "" {
				out = append(out, story{ci, it})
			}
		}
	}
	rng := rand.New(rand.NewPCG(seed, 0x6e6f637465726d)) // "nocterm"
	rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out[:min(len(out), maxStories)]
}

type readerPage struct {
	lines []string
	chars int
}

type readerPlan struct {
	story story
	head  []string
	pages []readerPage
	dur   time.Duration
}

func pageDur(p readerPage, first bool) time.Duration {
	d := time.Duration(p.chars)*time.Second/typeRate + readerHold
	if first {
		d += readerIntro
	}
	return d
}

// planReader wraps every story for a body w wide and h tall.
func planReader(ss []story, w, h int) ([]readerPlan, time.Duration) {
	var plans []readerPlan
	var total time.Duration
	for _, s := range ss {
		p := readerPlan{story: s, head: wrap(s.item.Title, w, 2)}
		lines := wrapAll(s.item.Summary, w)
		for i := 0; i < len(lines); i += h {
			pg := readerPage{lines: lines[i:min(i+h, len(lines))]}
			for _, l := range pg.lines {
				pg.chars += len(l)
			}
			p.pages = append(p.pages, pg)
			p.dur += pageDur(pg, i == 0)
		}
		if len(p.pages) == 0 {
			continue
		}
		plans = append(plans, p)
		total += p.dur
	}
	return plans, total
}

// The plan only changes when the news, or the reader's size, does.
var readerCache struct {
	sync.Mutex
	news  *feeds.News
	w, h  int
	plans []readerPlan
	total time.Duration
}

func cachedPlan(n *feeds.News, w, h int) ([]readerPlan, time.Duration) {
	readerCache.Lock()
	defer readerCache.Unlock()
	if readerCache.news != n || readerCache.w != w || readerCache.h != h {
		readerCache.plans, readerCache.total = planReader(stories(n, uint64(n.Generated.UnixNano())), w, h)
		readerCache.news, readerCache.w, readerCache.h = n, w, h
	}
	return readerCache.plans, readerCache.total
}

// readerPos is where the reader is at a moment.
type readerPos struct {
	plan, page int
	shown      int  // characters of the page typed so far
	typed      bool // page fully typed
}

// locate finds the story, page and typing progress el into the cycle.
func locate(plans []readerPlan, total, el time.Duration) readerPos {
	if len(plans) == 0 || total <= 0 {
		return readerPos{}
	}
	el %= total
	if el < 0 {
		el += total
	}
	for pi, p := range plans {
		if el >= p.dur {
			el -= p.dur
			continue
		}
		for gi, pg := range p.pages {
			d := pageDur(pg, gi == 0)
			if el >= d {
				el -= d
				continue
			}
			if gi == 0 {
				el -= readerIntro
			}
			shown := max(0, int(el*typeRate/time.Second))
			return readerPos{plan: pi, page: gi, shown: min(shown, pg.chars), typed: shown >= pg.chars}
		}
	}
	return readerPos{}
}

func drawReader(r canvas.Region, v *View, t *Theme) {
	n := v.Feeds.News
	if n == nil {
		in := Frame(r, t, "Reader", "", t.Host)
		feedStatus(in, t, v.Feeds.NewsOn, v.Feeds.NewsErr, "news")
		return
	}
	// Graphics tiles draw the reader at double size; terminals cannot.
	sc := 1
	if r.Square() {
		sc = 2
	}
	// Body geometry in characters: below a two-line headline, a meta
	// line and a gap.
	headRows := 2 * sc
	bw := min((r.W-4)/sc, readerWidth)
	bh := (r.H - 2 - headRows - 2) / sc
	if bw < 10 || bh < 1 {
		Frame(r, t, "Reader", "", t.Host)
		return
	}
	plans, total := cachedPlan(n, bw, bh)
	if len(plans) == 0 {
		in := Frame(r, t, "Reader", "", t.Host)
		in.Text(0, 0, "no summaries in this feed", t.Dim)
		return
	}
	el := v.Now.Sub(v.Feeds.NewsAt)
	if v.Reader.Set {
		el = v.Reader.Elapsed
	}
	pos := locate(plans, total, el)
	p := plans[pos.plan]
	cat := n.Categories[p.story.cat]
	accent := t.accent(p.story.cat)
	sub := fmt.Sprintf("story %d/%d", pos.plan+1, len(plans))
	if len(p.pages) > 1 {
		sub += fmt.Sprintf(" · page %d/%d", pos.page+1, len(p.pages))
	}
	if v.Reader.Paused {
		sub = "PAUSED · space resumes · " + sub
	}
	in := Frame(r, t, cat.Name, sub, accent)
	if in.Empty() {
		return
	}
	x0 := (in.W - bw*sc) / 2
	put := func(y int, s string, fg canvas.Color, bold bool) {
		switch {
		case sc == 2:
			in.Double(x0, y, s, fg, bold)
		case bold:
			in.Bold(x0, y, s, fg)
		default:
			in.Text(x0, y, s, fg)
		}
	}

	for i, l := range p.head {
		put(i*sc, l, t.FG, true)
	}
	age, ageCol := ageOf(p.story.item.Published, v.Now, t, accent)
	mx := in.Bold(x0, headRows, age, ageCol)
	in.Text(mx+1, headRows, "· "+p.story.item.Feed, t.Dim)

	left := pos.shown
	for i, l := range p.pages[pos.page].lines {
		if left <= 0 {
			break
		}
		put(headRows+2+i*sc, l[:min(len(l), left)], t.FG, false)
		left -= len(l)
	}
	// The RPG "more" prompt: blinks once a page is typed and more follow.
	if pos.typed && pos.page < len(p.pages)-1 && v.Now.UnixMilli()/500%2 == 0 {
		in.Set(x0+bw*sc-1, in.H-1, '▼', accent)
	}
}

// start returns when story i begins within the cycle.
func start(plans []readerPlan, i int) time.Duration {
	var d time.Duration
	for _, p := range plans[:i] {
		d += p.dur
	}
	return d
}

// wrapAll wraps s to width w with no line limit; words longer than w are
// split.
func wrapAll(s string, w int) []string {
	if w <= 0 {
		return nil
	}
	var out []string
	cur := ""
	for _, word := range strings.Fields(s) {
		for len(word) > w {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			out = append(out, word[:w])
			word = word[w:]
		}
		switch {
		case cur == "":
			cur = word
		case len(cur)+1+len(word) <= w:
			cur += " " + word
		default:
			out = append(out, cur)
			cur = word
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// ReaderClock is the reader's pausable clock. Run loops own one, toggle it
// on the space key, and Apply it to each View. While paused it also holds
// on to the news being read, so a feed refresh cannot swap the story out
// from under the reader.
type ReaderClock struct {
	paused   bool
	pausedAt time.Time
	offset   time.Duration // total time spent paused since the news arrived
	news     *feeds.News
	newsAt   time.Time
}

// Toggle pauses or resumes at now.
func (c *ReaderClock) Toggle(now time.Time) {
	if c.paused {
		c.offset += now.Sub(c.pausedAt)
		c.paused = false
		return
	}
	c.paused, c.pausedAt = true, now
}

// Skip jumps delta stories (negative goes back) from the one showing at
// now, to the start of that story, and resumes if paused: landing on a
// paused story would show only its headline. It uses the plan of the most
// recent frame; before any frame it does nothing.
func (c *ReaderClock) Skip(delta int, now time.Time) {
	readerCache.Lock()
	plans, total := readerCache.plans, readerCache.total
	news := readerCache.news
	readerCache.Unlock()
	if len(plans) == 0 || total <= 0 || news != c.news {
		return
	}
	if c.paused {
		c.Toggle(now)
	}
	el := now.Sub(c.newsAt) - c.offset
	cyc := el % total
	if cyc < 0 {
		cyc += total
	}
	cur := locate(plans, total, el).plan
	next := ((cur+delta)%len(plans) + len(plans)) % len(plans)
	target := el - cyc + start(plans, next)
	// Going forward past the last story wraps into the next cycle, and
	// back from the first into the previous one, so time moves the
	// expected way.
	if delta > 0 && next <= cur {
		target += total
	} else if delta < 0 && next >= cur {
		target -= total
	}
	c.offset += el - target
}

// Paused reports whether the reader is paused.
func (c *ReaderClock) Paused() bool { return c.paused }

// Apply sets v's reader position from the clock.
func (c *ReaderClock) Apply(v *View) {
	if c.paused {
		if c.news != nil {
			v.Feeds.News, v.Feeds.NewsAt = c.news, c.newsAt
		}
		v.Reader = ReaderState{Set: true, Paused: true, Elapsed: c.pausedAt.Sub(c.newsAt) - c.offset}
		return
	}
	if v.Feeds.News != c.news {
		// New news restarts the cycle from its first story.
		c.news, c.newsAt, c.offset = v.Feeds.News, v.Feeds.NewsAt, 0
	}
	v.Reader = ReaderState{Set: true, Elapsed: v.Now.Sub(c.newsAt) - c.offset}
}

// ReaderState is where the reader is, when a run loop drives it.
type ReaderState struct {
	Set     bool
	Paused  bool
	Elapsed time.Duration
}
