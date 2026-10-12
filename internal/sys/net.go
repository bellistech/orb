package sys

import (
	"os"
	"strings"
)

// NetIf is one interface's current throughput in bytes/second.
type NetIf struct {
	Name           string
	Up             bool
	RxRate, TxRate float64
}

type netCounters struct{ rx, tx uint64 }

var virtualPrefixes = []string{"veth", "docker", "br-", "virbr", "cni", "lo"}

// readNet returns byte counters for interfaces worth showing: physical
// devices always, virtual ones unless they are container plumbing.
func readNet() map[string]netCounters {
	out := map[string]netCounters{}
	lines := strings.Split(readString("/proc/net/dev"), "\n")
	for _, line := range lines {
		name, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		if skipIf(name) {
			continue
		}
		f := strings.Fields(rest)
		if len(f) < 9 {
			continue
		}
		out[name] = netCounters{rx: atou(f[0]), tx: atou(f[8])}
	}
	return out
}

func skipIf(name string) bool {
	if _, err := os.Stat("/sys/class/net/" + name + "/device"); err == nil {
		return false
	}
	for _, p := range virtualPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func ifUp(name string) bool {
	return readString("/sys/class/net/"+name+"/operstate") == "up"
}
