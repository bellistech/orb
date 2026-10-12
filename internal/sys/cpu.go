package sys

import (
	"fmt"
	"strings"
)

type cpuTimes struct{ idle, total uint64 }

// readCPUTimes parses /proc/stat. Index 0 is the aggregate, 1.. are cores.
func readCPUTimes() []cpuTimes {
	var out []cpuTimes
	for _, line := range strings.Split(readString("/proc/stat"), "\n") {
		if !strings.HasPrefix(line, "cpu") {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 9 {
			continue
		}
		// user nice system idle iowait irq softirq steal; guest is
		// already counted in user.
		var t cpuTimes
		for i := 1; i <= 8; i++ {
			t.total += atou(f[i])
		}
		t.idle = atou(f[4]) + atou(f[5])
		out = append(out, t)
	}
	return out
}

func busy(prev, cur cpuTimes) float64 {
	dt := float64(cur.total - prev.total)
	if cur.total <= prev.total || dt == 0 {
		return 0
	}
	di := float64(cur.idle - prev.idle)
	return clamp01(1 - di/dt)
}

// coreFreqMHz returns the current frequency of core i, or 0.
func coreFreqMHz(i int) float64 {
	khz, ok := readUint(fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/scaling_cur_freq", i))
	if !ok {
		return 0
	}
	return float64(khz) / 1000
}

func clamp01(v float64) float64 { return max(0, min(1, v)) }
