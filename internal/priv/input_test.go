package priv

import (
	"io"
	"strings"
	"testing"
	"time"
)

func TestPumpInput(t *testing.T) {
	r, w := io.Pipe()
	out := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() { done <- pumpInput(r, out) }()
	select {
	case <-out:
		t.Fatal("signal before any tick")
	case <-time.After(20 * time.Millisecond):
	}
	for range 3 {
		if _, err := w.Write([]byte{inputTick}); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-out:
	case <-time.After(2 * time.Second):
		t.Fatal("no signal after ticks")
	}
	// The helper ending ends the pump.
	w.Close()
	select {
	case err := <-done:
		if err != io.EOF {
			t.Fatalf("pump ended with %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pump still running")
	}
}

// Anything but a tick ends the pump: the main process takes no data from
// the helper, not even by accident.
func TestPumpInputRejects(t *testing.T) {
	for _, bad := range []string{"x", ".k.", "..\n", "R\n", "\x00"} {
		out := make(chan struct{}, 1)
		err := pumpInput(strings.NewReader(bad), out)
		if err == nil || err == io.EOF {
			t.Errorf("pumpInput(%q) = %v", bad, err)
		}
	}
}
