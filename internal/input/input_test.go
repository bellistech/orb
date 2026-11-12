package input

import (
	"os"
	"testing"
	"time"
)

func TestHuman(t *testing.T) {
	for types, want := range map[byte]bool{
		0:                          false,
		1 << 0:                     false, // EV_SYN alone
		1<<0 | 1<<evKey:            true,  // keyboard, power button
		1<<0 | 1<<evKey | 1<<evRel: true,  // mouse
		1<<0 | 1<<evRel:            true,  // wheel
		1<<0 | 1<<3:                false, // EV_ABS alone: accelerometer
		1<<0 | 1<<5:                false, // EV_SW: lid switch
		1<<0 | 1<<evKey | 1<<3:     true,  // touchpad
	} {
		if got := human(types); got != want {
			t.Errorf("human(%#b) = %v, want %v", types, got, want)
		}
	}
}

func TestWatch(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	out := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		Watch(r, out, time.Millisecond)
		close(done)
	}()
	select {
	case <-out:
		t.Fatal("signal with no input")
	case <-time.After(20 * time.Millisecond):
	}
	// A burst is one signal, not a backlog.
	for range 5 {
		if _, err := w.Write(make([]byte, 24)); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-out:
	case <-time.After(2 * time.Second):
		t.Fatal("no signal after input")
	}
	// The watcher ends when the device goes away.
	w.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher still running after close")
	}
}
