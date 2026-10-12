package sys

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Host is identity and load.
type Host struct {
	Hostname string
	OS       string
	Kernel   string
	CPUModel string
	Uptime   time.Duration
	Load     [3]float64
	Running  int
	Tasks    int
}

func readHostStatic() Host {
	h := Host{Kernel: readString("/proc/sys/kernel/osrelease")}
	h.Hostname, _ = os.Hostname()
	for _, line := range strings.Split(readString("/etc/os-release"), "\n") {
		if v, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
			h.OS = strings.Trim(v, `"`)
		}
	}
	for _, line := range strings.Split(readString("/proc/cpuinfo"), "\n") {
		if k, v, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(k) == "model name" {
			h.CPUModel = strings.TrimSpace(v)
			break
		}
	}
	return h
}

func readHostDynamic(h *Host) {
	if f := strings.Fields(readString("/proc/uptime")); len(f) > 0 {
		if s, err := strconv.ParseFloat(f[0], 64); err == nil {
			h.Uptime = time.Duration(s) * time.Second
		}
	}
	f := strings.Fields(readString("/proc/loadavg"))
	if len(f) >= 4 {
		for i := range 3 {
			h.Load[i], _ = strconv.ParseFloat(f[i], 64)
		}
		if r, t, ok := strings.Cut(f[3], "/"); ok {
			h.Running, _ = strconv.Atoi(r)
			h.Tasks, _ = strconv.Atoi(t)
		}
	}
}
