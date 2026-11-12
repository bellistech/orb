package gfx

import (
	"testing"
	"time"
)

func TestCheckBlank(t *testing.T) {
	for d, want := range map[time.Duration]bool{
		0:                      true,
		5 * time.Minute:        true,
		BlankMin:               true,
		BlankMax:               true,
		BlankMin - time.Second: false,
		BlankMax + time.Second: false,
		-time.Minute:           false,
	} {
		if got := CheckBlank(d); got != want {
			t.Errorf("CheckBlank(%v) = %v, want %v", d, got, want)
		}
	}
}

func TestBlankerNever(t *testing.T) {
	t0 := time.Unix(1000, 0)
	b := NewBlanker(0, t0)
	if b.Step(t0.Add(100 * time.Hour)) {
		t.Fatal("blanked with blanking off")
	}
	if b.Input(t0.Add(101*time.Hour)) || b.Off() || b.Waking(t0.Add(101*time.Hour)) {
		t.Fatal("input woke a screen that never slept")
	}
}

func TestBlankerCycle(t *testing.T) {
	t0 := time.Unix(1000, 0)
	b := NewBlanker(5*time.Minute, t0)
	if b.Step(t0.Add(5*time.Minute - time.Second)) {
		t.Fatal("blanked early")
	}
	// Input pushes the deadline out and wakes nothing.
	if b.Input(t0.Add(4 * time.Minute)) {
		t.Fatal("input woke a lit screen")
	}
	if b.Waking(t0.Add(4 * time.Minute)) {
		t.Fatal("a key on a lit screen was swallowed")
	}
	if b.Step(t0.Add(8 * time.Minute)) {
		t.Fatal("blanked 4 minutes after input")
	}
	if !b.Step(t0.Add(9 * time.Minute)) {
		t.Fatal("not blanked 5 minutes after input")
	}
	if !b.Off() {
		t.Fatal("Off() false after blanking")
	}
	// The transition is reported once.
	if b.Step(t0.Add(10 * time.Minute)) {
		t.Fatal("blanked twice")
	}
	// Input wakes; a key right behind it is part of the wake-up, a later
	// one is not.
	w := t0.Add(30 * time.Minute)
	if !b.Input(w) {
		t.Fatal("input did not wake")
	}
	if b.Off() {
		t.Fatal("Off() true after waking")
	}
	if !b.Waking(w) || !b.Waking(w.Add(wakeGrace-time.Millisecond)) {
		t.Fatal("the waking key would act")
	}
	if b.Waking(w.Add(wakeGrace)) {
		t.Fatal("a key after the grace time was swallowed")
	}
	// Idle counts again from the wake-up.
	if b.Step(w.Add(5*time.Minute - time.Second)) {
		t.Fatal("blanked early after waking")
	}
	if !b.Step(w.Add(5 * time.Minute)) {
		t.Fatal("not blanked again")
	}
}
