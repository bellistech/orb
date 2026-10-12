// Package tui runs the dashboard as a Bubble Tea program.
package tui

import (
	"image/color"
	"time"

	tea "charm.land/bubbletea/v2"

	"nocterm/internal/canvas"
	"nocterm/internal/sys"
	"nocterm/internal/ui"
)

type sampleMsg *sys.Snapshot

// Model is the Bubble Tea model. It only displays; the sole keys are t
// (next theme) and q (quit).
type Model struct {
	source   func() *sys.Snapshot
	interval time.Duration
	theme    int
	snap     *sys.Snapshot
	w, h     int
}

// New returns a model sampling every interval, starting on theme. With demo
// it shows synthetic data instead of the host's.
func New(interval time.Duration, theme string, demo bool) Model {
	m := Model{interval: interval}
	if demo {
		m.source = func() *sys.Snapshot { return sys.Demo(time.Now()) }
	} else {
		m.source = sys.NewSampler().Sample
	}
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
func (m Model) Init() tea.Cmd { return m.sample(250 * time.Millisecond) }

// Update handles samples, resizes and the two keys.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case sampleMsg:
		m.snap = msg
		// Exactly one sample is ever in flight: the next is scheduled
		// only after this one lands.
		return m, m.sample(m.interval)
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "t":
			m.theme = (m.theme + 1) % len(ui.Themes)
		}
	}
	return m, nil
}

// View renders the current snapshot full-screen.
func (m Model) View() tea.View {
	t := ui.Themes[m.theme]
	v := tea.NewView(ui.Render(m.w, m.h, m.snap, t).String())
	v.AltScreen = true
	v.WindowTitle = "nocterm"
	v.BackgroundColor = rgba(t.BG)
	return v
}

func rgba(c canvas.Color) color.Color { return color.RGBA{R: c.R, G: c.G, B: c.B, A: 0xff} }
