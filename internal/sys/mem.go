package sys

import "strings"

// Mem is memory usage in bytes.
type Mem struct {
	Total, Used, Available, Cached, Buffers uint64
	SwapTotal, SwapUsed                     uint64
}

// UsedFrac is used/total in [0,1].
func (m Mem) UsedFrac() float64 { return frac(m.Used, m.Total) }

func readMem() Mem {
	kv := map[string]uint64{}
	for _, line := range strings.Split(readString("/proc/meminfo"), "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		f := strings.Fields(v)
		if len(f) == 0 {
			continue
		}
		kv[k] = atou(f[0]) * 1024
	}
	m := Mem{
		Total:     kv["MemTotal"],
		Available: kv["MemAvailable"],
		Cached:    kv["Cached"] + kv["SReclaimable"],
		Buffers:   kv["Buffers"],
		SwapTotal: kv["SwapTotal"],
	}
	if m.Total > m.Available {
		m.Used = m.Total - m.Available
	}
	if free := kv["SwapFree"]; m.SwapTotal > free {
		m.SwapUsed = m.SwapTotal - free
	}
	return m
}

func frac(a, b uint64) float64 {
	if b == 0 {
		return 0
	}
	return clamp01(float64(a) / float64(b))
}
