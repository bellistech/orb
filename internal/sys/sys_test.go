package sys

import (
	"reflect"
	"runtime"
	"testing"
)

func TestRingWraps(t *testing.T) {
	r := NewRing(3)
	for i := 1; i <= 5; i++ {
		r.Push(float64(i))
	}
	if got := r.Values(); !reflect.DeepEqual(got, []float64{3, 4, 5}) {
		t.Fatalf("got %v", got)
	}
}

func TestBusy(t *testing.T) {
	if got := busy(cpuTimes{idle: 50, total: 100}, cpuTimes{idle: 75, total: 200}); got != 0.75 {
		t.Fatalf("got %v, want 0.75", got)
	}
	// A counter that went backwards must not produce garbage.
	if got := busy(cpuTimes{total: 200}, cpuTimes{total: 100}); got != 0 {
		t.Fatalf("got %v, want 0", got)
	}
}

func TestUnescapeMount(t *testing.T) {
	for in, want := range map[string]string{
		`/mnt/my\040disk`: "/mnt/my disk",
		`/plain`:          "/plain",
		`/bad\09x`:        `/bad\09x`,
		`/end\04`:         `/end\04`,
	} {
		if got := unescapeMount(in); got != want {
			t.Errorf("unescapeMount(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTopProcs(t *testing.T) {
	prev := map[int]procTicks{1: {ticks: 100}, 2: {ticks: 100}}
	cur := map[int]procTicks{1: {ticks: 150, name: "a"}, 2: {ticks: 300, name: "b"}, 3: {ticks: 999, name: "new", rss: 5}}
	got := topProcs(prev, cur, 1, 2)
	if len(got) != 2 || got[0].Name != "b" || got[0].CPU != 2 || got[1].Name != "a" {
		t.Fatalf("got %+v", got)
	}
}

func TestSampleOnThisHost(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reads /proc")
	}
	s := NewSampler().Sample()
	if len(s.Cores) == 0 || s.Mem.Total == 0 || s.Host.Kernel == "" {
		t.Fatalf("empty snapshot: cores=%d mem=%d kernel=%q", len(s.Cores), s.Mem.Total, s.Host.Kernel)
	}
}
