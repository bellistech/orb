package sys

// Ring is a fixed-size history of float samples.
type Ring struct {
	buf  []float64
	head int
	n    int
}

// NewRing returns a ring holding up to size samples.
func NewRing(size int) *Ring { return &Ring{buf: make([]float64, max(size, 1))} }

// Push appends v, dropping the oldest sample when full.
func (r *Ring) Push(v float64) {
	r.buf[r.head] = v
	r.head = (r.head + 1) % len(r.buf)
	r.n = min(r.n+1, len(r.buf))
}

// Values returns a copy of the samples, oldest first.
func (r *Ring) Values() []float64 {
	out := make([]float64, r.n)
	start := (r.head - r.n + len(r.buf)) % len(r.buf)
	for i := range out {
		out[i] = r.buf[(start+i)%len(r.buf)]
	}
	return out
}
