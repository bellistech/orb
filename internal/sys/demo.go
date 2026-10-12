package sys

import (
	"math"
	"time"
)

// Demo returns a synthetic snapshot of a busy 12-thread box with full
// histories, for design work and screenshots. Nothing is read from the host.
func Demo(now time.Time) *Snapshot {
	t := float64(now.UnixMilli()) / 1000
	wave := func(i, phase, period, lo, hi float64) float64 {
		v := (math.Sin((t-float64(History)+i)/period+phase) + 1) / 2
		v = v*0.75 + 0.25*(math.Sin((t+i)*1.7+phase*3)+1)/2
		return lo + (hi-lo)*v
	}
	hist := func(phase, period, lo, hi float64) []float64 {
		out := make([]float64, History)
		for i := range out {
			out[i] = wave(float64(i), phase, period, lo, hi)
		}
		return out
	}
	last := func(v []float64) float64 { return v[len(v)-1] }

	s := &Snapshot{
		Time: now,
		Host: Host{
			Hostname: "west", OS: "Ubuntu 25.10", Kernel: "6.17.0-41-generic",
			CPUModel: "AMD Ryzen 5 7600X 6-Core Processor",
			Uptime:   3*24*time.Hour + 4*time.Hour + 12*time.Minute,
			Load:     [3]float64{7.82, 6.10, 4.95}, Running: 9, Tasks: 612,
		},
	}
	s.CPUHist = hist(0, 18, 0.05, 0.92)
	s.CPU = last(s.CPUHist)
	for i := range 12 {
		h := hist(float64(i)*0.9, 9+float64(i), 0.02, 1)
		s.CoreHist = append(s.CoreHist, h)
		s.Cores = append(s.Cores, last(h))
		s.CoreMHz = append(s.CoreMHz, 3000+1700*last(h))
	}
	const gib = 1 << 30
	s.MemHist = hist(1, 60, 0.35, 0.78)
	total := uint64(64 * gib)
	used := uint64(last(s.MemHist) * float64(total))
	s.Mem = Mem{Total: total, Used: used, Available: total - used, Cached: 18 * gib, Buffers: gib, SwapTotal: 8 * gib, SwapUsed: gib / 3}

	s.NetRx = hist(2, 7, 2e5, 9e7)
	s.NetTx = hist(4, 11, 5e4, 2.2e7)
	s.Net = []NetIf{
		{Name: "enp16s0", Up: true, RxRate: last(s.NetRx) * 0.9, TxRate: last(s.NetTx) * 0.8},
		{Name: "wg0", Up: true, RxRate: last(s.NetRx) * 0.1, TxRate: last(s.NetTx) * 0.2},
		{Name: "wlp17s0", Up: false},
	}
	s.DiskRead = hist(3, 5, 0, 4.2e8)
	s.DiskWrite = hist(5, 13, 0, 1.6e8)
	s.Mounts = []Mount{
		{Path: "/", Size: 334 * gib, Used: 290 * gib, Avail: 44 * gib},
		{Path: "/var", Size: 1800 * gib, Used: 610 * gib, Avail: 1190 * gib},
		{Path: "/boot", Size: 2 * gib, Used: gib / 4, Avail: 7 * gib / 4},
		{Path: "/srv/backup", Size: 3600 * gib, Used: 3400 * gib, Avail: 200 * gib},
	}
	s.DiskIO = []DiskIO{
		{Name: "nvme0n1", ReadRate: last(s.DiskRead), WriteRate: last(s.DiskWrite)},
		{Name: "sda", ReadRate: last(s.DiskRead) / 9, WriteRate: 0},
	}
	s.Temps = []Temp{
		{Chip: "cpu", Label: "Tctl", Celsius: 40 + 50*s.CPU},
		{Chip: "cpu", Label: "Tccd1", Celsius: 36 + 46*s.CPU},
		{Chip: "gpu", Label: "edge", Celsius: 58},
		{Chip: "gpu", Label: "junction", Celsius: 71},
		{Chip: "gpu", Label: "mem", Celsius: 66},
		{Chip: "nvme", Label: "Composite", Celsius: 47},
		{Chip: "dimm", Celsius: 41},
	}
	s.Fans = []Fan{{Chip: "gpu", RPM: 1480}}
	g := hist(6, 6, 0.1, 0.98)
	s.GPUs = []GPU{
		{Card: "card1", Discrete: true, Busy: last(g), VRAMUsed: 9 * gib, VRAMTotal: 12 * gib, PowerW: 40 + 180*last(g), SclkMHz: 2450, TempC: 58},
		{Card: "card0", Busy: 0.04, GTTUsed: gib, GTTTotal: 7 * gib, PowerW: 12, SclkMHz: 600, TempC: 44},
	}
	s.GPUHist = [][]float64{g, hist(7, 30, 0, 0.1)}
	names := []string{"llama-server", "go", "rustc", "postgres", "prometheus", "haproxy", "dockerd", "wotan", "sshd", "systemd-journal"}
	for i, n := range names {
		s.Procs = append(s.Procs, Proc{PID: 1200 + i*331, Name: n, CPU: 3.4 / float64(i+1) * (0.6 + 0.4*s.CPU), RSS: uint64(9-i) * gib / 3})
	}
	return s
}
