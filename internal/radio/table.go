package radio

import (
	"sort"
	"sync"
	"time"
)

// maxAPs bounds the table: a crowded street has a few hundred.
const maxAPs = 1024

// Table remembers networks after the kernel forgets them (it drops a BSS
// about 30 s after its last beacon), so the page shows what was heard in
// the last Keep. It is safe for concurrent use.
type Table struct {
	Keep time.Duration
	mu   sync.Mutex
	aps  map[[6]byte]AP
}

// NewTable keeps networks for keep after they were last heard.
func NewTable(keep time.Duration) *Table {
	return &Table{Keep: keep, aps: map[[6]byte]AP{}}
}

// Heard records a dump. A network's newest sighting wins; one not in the
// dump keeps its last values.
func (t *Table) Heard(aps []AP) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, a := range aps {
		if old, ok := t.aps[a.BSSID]; ok && old.Seen.After(a.Seen) {
			continue
		}
		if _, ok := t.aps[a.BSSID]; !ok && len(t.aps) >= maxAPs {
			continue
		}
		t.aps[a.BSSID] = a
	}
}

// List drops what is older than Keep and returns the rest: the joined
// network first, then strongest first, then by name.
func (t *Table) List(now time.Time) []AP {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]AP, 0, len(t.aps))
	for k, a := range t.aps {
		if now.Sub(a.Seen) > t.Keep {
			delete(t.aps, k)
			continue
		}
		out = append(out, a)
	}
	Sort(out)
	return out
}

// Sort orders networks for display.
func Sort(aps []AP) {
	sort.Slice(aps, func(i, j int) bool {
		a, b := aps[i], aps[j]
		if a.Joined != b.Joined {
			return a.Joined
		}
		if a.Signal != b.Signal {
			return a.Signal > b.Signal
		}
		if a.SSID != b.SSID {
			return a.SSID < b.SSID
		}
		return MAC(a.BSSID) < MAC(b.BSSID)
	})
}
