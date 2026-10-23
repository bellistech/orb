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

// Model is the Bubble Tea model. It only displays; the keys are d, w and n
// l (pages), space (pause the news reader), arrows (previous/next story),
// t (next theme) and q (quit).
type Model struct {
	source   func() *sys.Snapshot
	interval time.Duration
	theme    int
	page     ui.Page
	feeds    func() feeds.View
	reader   *ui.ReaderClock // pointer: the clock outlives Model copies
	onPage   func(ui.Page)
	snap     *sys.Snapshot
	w, h     int
}

// New returns a model sampling source every interval, starting on theme and
// page. source must be the one main configured (socket and link demand,
// the root helper's owner lookup); onPage is told every page shown, so the
// source reads sockets or links only while their page is up. feedView and
// onPage may be nil.
func New(interval time.Duration, theme string, page ui.Page, source func() *sys.Snapshot, feedView func() feeds.View, onPage func(ui.Page)) Model {
	if feedView == nil {
		feedView = func() feeds.View { return feeds.View{} }
	}
	if onPage == nil {
		onPage = func(ui.Page) {}
	}
	onPage(page)
	m := Model{interval: interval, page: page, feeds: feedView, reader: &ui.ReaderClock{}, onPage: onPage, source: source}
	for i, t := range ui.Themes {
		if t.Name == theme {
			m.theme = i
		}
	}
	return m
}

func (m Model) sample(d time.Duration) tea.Cmd {
	src := m.source
	return tea.Tick(d, func(time.Time) tea.Msg { return sampleMsg(src()) })
}

// Init takes the first sample shortly after start so rates have a baseline.
func (m Model) Init() tea.Cmd {
	if m.page == ui.News {
		return tea.Batch(m.sample(250*time.Millisecond), frameTick())
	}
	return m.sample(250 * time.Millisecond)
}

// Update handles samples, resizes and the two keys.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case sampleMsg:
		m.snap = msg
		// Exactly one sample is ever in flight: the next is scheduled
		// only after this one lands.
		return m, m.sample(m.interval)
	case frameMsg:
		// The tick chain stops by itself once the page changes.
		if m.page == ui.News {
			return m, frameTick()
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "left":
			m.reader.Skip(-1, time.Now())
		case "down", "right":
			m.reader.Skip(1, time.Now())
		case "t":
			m.theme = (m.theme + 1) % len(ui.Themes)
		case "d":
			m.page = ui.Dash
			m.onPage(m.page)
		case "n":
			m.page = ui.Net
			m.onPage(m.page)
		case "l":
			m.page = ui.Links
			m.onPage(m.page)
		case "space", " ":
			m.reader.Toggle(time.Now())
		case "w":
			m.onPage(ui.News)
			if m.page != ui.News {
				m.page = ui.News
				return m, frameTick()
			}
		}
	}
	return m, nil
}

// View renders the current snapshot full-screen.
func (m Model) View() tea.View {
	t := ui.Themes[m.theme]
	view := ui.View{Page: m.page, Snap: m.snap, Feeds: m.feeds(), Now: time.Now()}
	m.reader.Apply(&view)
	v := tea.NewView(ui.Render(m.w, m.h, view, t, false).String())
	v.AltScreen = true
	v.WindowTitle = "orb"
	v.BackgroundColor = rgba(t.BG)
	return v
}

func rgba(c canvas.Color) color.Color { return color.RGBA{R: c.R, G: c.G, B: c.B, A: 0xff} }
