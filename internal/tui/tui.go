// Package tui runs the dashboard as a Bubble Tea program.
package tui

import (
	"image/color"
	"time"

	tea "charm.land/bubbletea/v2"

	"orb/internal/canvas"
	"orb/internal/feeds"
	"orb/internal/sys"
	"orb/internal/ui"
)

type sampleMsg *sys.Snapshot

// frameMsg redraws without sampling, for the news page's typing.
type frameMsg struct{}

const frameEvery = 100 * time.Millisecond

func frameTick() tea.Cmd {
	return tea.Tick(frameEvery, func(time.Time) tea.Msg { return frameMsg{} })
}

// themeMsg redraws when the theme cycle's palette changes. gen tells
// chains apart: only the one started by the latest theme choice goes on.
type themeMsg struct{ gen int }

// Model is the Bubble Tea model. It only displays; the keys are d, w and n
// l (pages), space (pause the news reader), arrows (previous/next story),
// t (next theme), i (attract mode) and q (quit).
type Model struct {
	source   func() *sys.Snapshot
	interval time.Duration
	theme    ui.ThemeSel
	themeGen int
	page     ui.Page
	feeds    func() feeds.View
	reader   *ui.ReaderClock // pointer: the clock outlives Model copies
	attract  *ui.Attract     // likewise
	onPage   func(ui.Page)
	snap     *sys.Snapshot
	w, h     int
}

// New returns a model sampling source every interval, starting on theme and
// page. source must be the one main configured (socket and link demand,
// the root helper's owner lookup); onPage is told every page shown, so the
// source reads sockets or links only while their page is up. feedView and
// onPage may be nil. attract arms attract mode from the start.
func New(interval time.Duration, theme string, page ui.Page, source func() *sys.Snapshot, feedView func() feeds.View, onPage func(ui.Page), attract bool) Model {
	if feedView == nil {
		feedView = func() feeds.View { return feeds.View{} }
	}
	if onPage == nil {
		onPage = func(ui.Page) {}
	}
	onPage(page)
	m := Model{interval: interval, page: page, feeds: feedView, reader: &ui.ReaderClock{}, attract: ui.NewAttract(attract), onPage: onPage, source: source}
	m.theme, _ = ui.SelectTheme(theme, time.Now())
	return m
}

// themeTick wakes the model when the theme cycle next changes palette, at
// most every frameEvery: a fade redraws the whole screen.
func (m Model) themeTick() tea.Cmd {
	d := m.theme.Wake(time.Now())
	if d <= 0 {
		return nil
	}
	d = max(d, frameEvery)
	gen := m.themeGen
	return tea.Tick(d, func(time.Time) tea.Msg { return themeMsg{gen} })
}

func (m Model) sample(d time.Duration) tea.Cmd {
	src := m.source
	return tea.Tick(d, func(time.Time) tea.Msg { return sampleMsg(src()) })
}

// Init takes the first sample shortly after start so rates have a baseline.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.sample(250 * time.Millisecond), m.themeTick()}
	if m.page == ui.News {
		cmds = append(cmds, frameTick())
	}
	return tea.Batch(cmds...)
}

// setPage shows p; the news page's typing needs frame ticks.
func (m *Model) setPage(p ui.Page) tea.Cmd {
	m.onPage(p)
	if p == m.page {
		return nil
	}
	m.page = p
	if p == ui.News {
		return frameTick()
	}
	return nil
}

// Update handles samples, resizes and keys.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case sampleMsg:
		m.snap = msg
		// Exactly one sample is ever in flight: the next is scheduled
		// only after this one lands. Attract mode steps on samples too.
		next := m.sample(m.interval)
		if p, ok := m.attract.Step(time.Now(), m.page); ok {
			return m, tea.Batch(next, m.setPage(p))
		}
		return m, next
	case frameMsg:
		// The tick chain stops by itself once the page changes.
		if m.page == ui.News {
			return m, frameTick()
		}
		return m, nil
	case themeMsg:
		if msg.gen != m.themeGen {
			return m, nil
		}
		return m, m.themeTick()
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case tea.KeyPressMsg:
		// Any key stops attract mode's cycle, then does its usual job.
		m.attract.Key(time.Now())
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "left":
			m.reader.Skip(-1, time.Now())
		case "down", "right":
			m.reader.Skip(1, time.Now())
		case "t":
			m.theme = m.theme.Next(time.Now())
			m.themeGen++
			return m, m.themeTick()
		case "d":
			return m, m.setPage(ui.Dash)
		case "n":
			return m, m.setPage(ui.Net)
		case "l":
			return m, m.setPage(ui.Links)
		case "space", " ":
			m.reader.Toggle(time.Now())
		case "i":
			m.attract.Toggle(time.Now())
		case "w":
			return m, m.setPage(ui.News)
		}
	}
	return m, nil
}

// View renders the current snapshot full-screen.
func (m Model) View() tea.View {
	t := m.theme.Theme(time.Now(), true)
	view := ui.View{Page: m.page, Snap: m.snap, Feeds: m.feeds(), Now: time.Now(), Attract: m.attract.State()}
	m.reader.Apply(&view)
	v := tea.NewView(ui.Render(m.w, m.h, view, t, false).String())
	v.AltScreen = true
	v.WindowTitle = "orb"
	v.BackgroundColor = rgba(t.BG)
	return v
}

func rgba(c canvas.Color) color.Color { return color.RGBA{R: c.R, G: c.G, B: c.B, A: 0xff} }
