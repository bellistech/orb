package priv

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
	"syscall"
	"testing"
	"time"

	"orb/internal/sys"
)

func TestRadioLineRoundTrip(t *testing.T) {
	now := time.Unix(time.Now().Unix()-30, 0) // pumpRadio checks against the wall clock
	var b bytes.Buffer
	for _, w := range []string{"", "busy", "down"} {
		if err := writeRadio(&b, "wlp7s0", now, w); err != nil {
			t.Fatal(err)
		}
	}
	scans := &RadioScans{every: time.Minute, last: map[string]sys.ScanInfo{}, alive: true}
	if err := pumpRadio(bufio.NewReader(&b), scans); err == nil || !strings.Contains(err.Error(), "EOF") {
		t.Fatalf("pump ended with %v, want EOF", err)
	}
	st := scans.State()
	if !st.On || st.Every != time.Minute || st.Last["wlp7s0"].Err != "down" || !st.Last["wlp7s0"].At.Equal(now) {
		t.Fatalf("state %+v", st)
	}
}

func TestReadRadioRejects(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	for name, line := range map[string]string{
		"not json":     "hello",
		"bad iface":    `{"if":"wl p","at":1800000000,"err":""}`,
		"long iface":   `{"if":"wlp7s0wlp7s0wlp7s0","at":1800000000,"err":""}`,
		"unknown word": `{"if":"wlp7s0","at":1800000000,"err":"rm -rf"}`,
		"no time":      `{"if":"wlp7s0","err":""}`,
		"future":       `{"if":"wlp7s0","at":1900000000,"err":""}`,
		"too long":     `{"if":"wlp7s0","at":1800000000,"err":"","x":"` + strings.Repeat("a", 300) + `"}`,
	} {
		if _, _, err := readRadio([]byte(line), now); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// A malformed line ends the pump.
	scans := &RadioScans{last: map[string]sys.ScanInfo{}}
	if err := pumpRadio(bufio.NewReader(strings.NewReader("junk\n")), scans); err == nil || strings.Contains(err.Error(), "EOF") {
		t.Fatalf("bad line ended with %v", err)
	}
}

func TestScanWord(t *testing.T) {
	for err, want := range map[error]string{
		nil: "", syscall.EBUSY: "busy", syscall.ENETDOWN: "down", syscall.EPERM: "denied",
		syscall.EOPNOTSUPP: "unsupported", syscall.EIO: "failed", errors.New("x"): "failed",
	} {
		if got := scanWord(err); got != want || !scanErrs[got] {
			t.Errorf("%v = %q, want %q", err, got, want)
		}
	}
}

func TestCheckRadioScan(t *testing.T) {
	for d, ok := range map[time.Duration]bool{0: true, 30 * time.Second: true, time.Hour: true, 29 * time.Second: false, 2 * time.Hour: false, -time.Second: false} {
		if CheckRadioScan(d) != ok {
			t.Errorf("%v: got %v", d, !ok)
		}
	}
}
