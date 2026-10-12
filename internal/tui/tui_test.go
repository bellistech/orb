package tui

import (
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
	m := New(time.Second, "phosphor", ui.Dash, source, nil, func(b bool) { wanted = append(wanted, b) })

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
