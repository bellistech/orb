package sys

import (
	"net"
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

func TestLerp(t *testing.T) {
	a := &Snapshot{CPU: 0.2, Cores: []float64{0, 1}, Mem: Mem{Used: 100}, Temps: []Temp{{Celsius: 40}}}
	b := &Snapshot{CPU: 0.6, Cores: []float64{1, 0}, Mem: Mem{Used: 300}, Temps: []Temp{{Celsius: 60}}}
	m := Lerp(a, b, 0.5)
	if m.CPU != 0.4 || m.Cores[0] != 0.5 || m.Mem.Used != 200 || m.Temps[0].Celsius != 50 {
		t.Fatalf("midpoint wrong: %+v", m)
	}
	if b.Cores[0] != 1 || b.Temps[0].Celsius != 60 {
		t.Fatal("Lerp mutated its input")
	}
	if Lerp(a, b, 1) != b || Lerp(nil, b, 0.5) != b || Lerp(a, nil, 0.5) != nil {
		t.Fatal("edge cases")
	}
	// Shape change (a core appeared) takes b's values unblended.
	c := &Snapshot{Cores: []float64{0.9, 0.9, 0.9}}
	if got := Lerp(a, c, 0.5); got.Cores[2] != 0.9 {
		t.Fatalf("shape change: %+v", got.Cores)
	}
}

func TestPickAddrs(t *testing.T) {
	mk := func(s string) net.Addr {
		ip, n, err := net.ParseCIDR(s)
		if err != nil {
			t.Fatal(err)
		}
		n.IP = ip
		return n
	}
	got := pickAddrs([]net.Addr{
		mk("fe80::1/64"), mk("2001:db8::5/64"), mk("192.168.1.20/24"),
		mk("127.0.0.1/8"), &net.IPAddr{IP: net.ParseIP("10.0.0.9")}, mk("10.44.0.1/24"),
	})
	want := []string{"192.168.1.20", "10.44.0.1", "2001:db8::5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if pickAddrs(nil) != nil {
		t.Fatal("nil input should give nil")
	}
}
