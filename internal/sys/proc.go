package sys

import (
	"os"
	"sort"
	"strconv"
	"strings"
)

// Proc is one process's resource use.
type Proc struct {
	PID   int
	Name  string
	CPU   float64 // fraction of one core
	RSS   uint64
	State byte
}

type procTicks struct {
	ticks uint64
	name  string
	rss   uint64
	state byte
}

// clkTck is USER_HZ, which is 100 on every Linux arch that matters.
const clkTck = 100

var pageSize = uint64(os.Getpagesize())

func readProcs() map[int]procTicks {
	out := map[int]procTicks{}
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return out
	}
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		s := readString("/proc/" + e.Name() + "/stat")
		// comm may contain spaces and parens; it ends at the last ')'.
		open, end := strings.IndexByte(s, '('), strings.LastIndexByte(s, ')')
		if open < 0 || end < open {
			continue
		}
		f := strings.Fields(s[end+1:])
		// f[0]=state(3) f[11]=utime(14) f[12]=stime(15) f[21]=rss(24)
		if len(f) < 22 {
			continue
		}
		out[pid] = procTicks{
			ticks: atou(f[11]) + atou(f[12]),
			name:  s[open+1 : end],
			rss:   atou(f[21]) * pageSize,
			state: f[0][0],
		}
	}
	return out
}

func topProcs(prev, cur map[int]procTicks, seconds float64, n int) []Proc {
	out := make([]Proc, 0, len(cur))
	for pid, c := range cur {
		p := Proc{PID: pid, Name: c.name, RSS: c.rss, State: c.state}
		if old, ok := prev[pid]; ok && c.ticks >= old.ticks && seconds > 0 {
			p.CPU = float64(c.ticks-old.ticks) / clkTck / seconds
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CPU != out[j].CPU {
			return out[i].CPU > out[j].CPU
		}
		return out[i].RSS > out[j].RSS
	})
	return out[:min(n, len(out))]
}
