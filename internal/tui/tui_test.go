package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"orb/internal/sys"
	"orb/internal/ui"
)

// The terminal UI must sample through the source main configured (socket
// demand, the root helper's owner lookup), never a sampler of its own.
func TestUsesTheGivenSourceAndSocketSwitch(t *testing.T) {
	calls := 0
	snap := &sys.Snapshot{Time: time.Unix(1, 0), Sockets: []sys.Socket{{Proto: "tcp", State: "LISTEN"}}}
	source := func() *sys.Snapshot { calls++; return snap }
	var wanted []bool
	m := New(time.Second, "phosphor", ui.Dash, source, nil, func(p ui.Page) { wanted = append(wanted, p == ui.Net) }, false)

	msg := m.Init()() // first sample
	if calls != 1 {
		t.Fatalf("given source called %d times, want 1", calls)
	}
	next, _ := m.Update(msg)
	m = next.(Model)
	if m.snap != snap {
		t.Fatal("model did not keep the given source's snapshot")
	}

	next, _ = m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	m = next.(Model)
	if m.page != ui.Net || len(wanted) == 0 || !wanted[len(wanted)-1] {
		t.Fatalf("n: page %v, socket demand %v", m.page, wanted)
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = next.(Model)
	if wanted[len(wanted)-1] {
		t.Fatal("d left socket reading on")
	}
}

// i arms attract mode; once idle it cycles on samples; any other key stops
// the cycle and still does its job; i again disarms.
func TestAttractKeys(t *testing.T) {
	snap := &sys.Snapshot{Time: time.Unix(1, 0)}
	m := New(time.Second, "phosphor", ui.Dash, func() *sys.Snapshot { return snap }, nil, nil, false)
	key := func(r rune) {
		t.Helper()
		next, _ := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = next.(Model)
	}
	if m.attract.State() != ui.AttractOff {
		t.Fatal("attract on by default")
	}
	key('i')
	if m.attract.State() != ui.AttractArmed {
		t.Fatalf("i: state %v", m.attract.State())
	}
	m.w, m.h = 215, 45
	next, _ := m.Update(sampleMsg(snap))
	m = next.(Model)
	if !strings.Contains(m.View().Content, "AUTO") {
		t.Fatal("no indicator while armed")
	}
	m.attract.Key(time.Now().Add(-time.Minute)) // a minute without keys
	next, cmd := m.Update(sampleMsg(snap))
	m = next.(Model)
	if m.page != ui.News || m.attract.State() != ui.AttractCycling || cmd == nil {
		t.Fatalf("idle: page %v state %v", m.page, m.attract.State())
	}
	key('n')
	if m.page != ui.Net || m.attract.State() != ui.AttractArmed {
		t.Fatalf("n while cycling: page %v state %v", m.page, m.attract.State())
	}
	key('i')
	if m.attract.State() != ui.AttractOff || m.page != ui.Net {
		t.Fatalf("i again: page %v state %v", m.page, m.attract.State())
	}
}

// t into the theme cycle starts a redraw chain; a later t ends it, and
// only the newest chain survives.
func TestThemeCycleTicks(t *testing.T) {
	source := func() *sys.Snapshot { return &sys.Snapshot{} }
	last := ui.Themes[len(ui.Themes)-1].Name
	m := New(time.Second, last, ui.Dash, source, nil, nil, false)
	next, cmd := m.Update(tea.KeyPressMsg{Code: 't', Text: "t"})
	m = next.(Model)
	if m.theme.Name() != ui.Cycle || cmd == nil {
		t.Fatalf("t from %s: theme %s, tick %v", last, m.theme.Name(), cmd != nil)
	}
	if _, cmd := m.Update(themeMsg{m.themeGen}); cmd == nil {
		t.Fatal("cycle tick chain stopped")
	}
	if _, cmd := m.Update(themeMsg{m.themeGen - 1}); cmd != nil {
		t.Fatal("stale tick chain kept going")
	}
	next, cmd = m.Update(tea.KeyPressMsg{Code: 't', Text: "t"})
	m = next.(Model)
	if m.theme.Name() != ui.Themes[0].Name || cmd != nil {
		t.Fatalf("t out of the cycle: theme %s, tick %v", m.theme.Name(), cmd != nil)
	}
	if New(time.Second, ui.Cycle, ui.Dash, source, nil, nil, false).themeTick() == nil {
		t.Fatal("starting in the cycle schedules no tick")
	}
}
