package sys

// Lerp returns b with its current values moved t of the way back toward a,
// so bars glide between samples instead of jumping. Histories and anything
// whose shape changed between samples are taken from b as is. t is clamped
// to [0,1]; t=1 returns b's values.
func Lerp(a, b *Snapshot, t float64) *Snapshot {
	if b == nil {
		return nil
	}
	if a == nil || t >= 1 {
		return b
	}
	t = max(t, 0)
	f := func(x, y float64) float64 { return x + (y-x)*t }
	u := func(x, y uint64) uint64 { return uint64(f(float64(x), float64(y))) }

	out := *b
	out.CPU = f(a.CPU, b.CPU)
	if len(a.Cores) == len(b.Cores) {
		out.Cores = make([]float64, len(b.Cores))
		for i := range b.Cores {
			out.Cores[i] = f(a.Cores[i], b.Cores[i])
		}
	}
	out.Mem.Used = u(a.Mem.Used, b.Mem.Used)
	out.Mem.Cached = u(a.Mem.Cached, b.Mem.Cached)
	out.Mem.SwapUsed = u(a.Mem.SwapUsed, b.Mem.SwapUsed)
	if len(a.GPUs) == len(b.GPUs) {
		out.GPUs = append([]GPU(nil), b.GPUs...)
		for i := range out.GPUs {
			out.GPUs[i].Busy = f(a.GPUs[i].Busy, b.GPUs[i].Busy)
			out.GPUs[i].VRAMUsed = u(a.GPUs[i].VRAMUsed, b.GPUs[i].VRAMUsed)
			out.GPUs[i].GTTUsed = u(a.GPUs[i].GTTUsed, b.GPUs[i].GTTUsed)
		}
	}
	if len(a.Temps) == len(b.Temps) {
		out.Temps = append([]Temp(nil), b.Temps...)
		for i := range out.Temps {
			out.Temps[i].Celsius = f(a.Temps[i].Celsius, b.Temps[i].Celsius)
		}
	}
	return &out
}
