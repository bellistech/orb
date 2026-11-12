// Package input tells whether someone is at the keyboard or the mouse,
// for the monitor blanker (ADR-0005). In graphics mode the console hands
// orb key bytes but nothing from the mouse, so orb listens to the
// kernel's event devices. Only the fact that something happened is used:
// events are never decoded, so nothing typed or clicked is read as such.
package input

import (
	"io"
	"time"
)

// Dir is where the kernel's event devices live.
const Dir = "/dev/input"

// Throttle is the least time between two activity signals from one
// device. A moving mouse sends a thousand events a second; ten wake-ups
// are plenty.
const Throttle = 100 * time.Millisecond

// Event types (<linux/input-event-codes.h>), as bits of a device's
// type bitmap.
const (
	evKey = 1 // keys and buttons
	evRel = 2 // relative motion: mice, wheels
)

// human reports whether a device with this event-type bitmap is a
// keyboard or a pointer: it has keys/buttons or relative axes. Lid
// switches, accelerometers and the PC speaker have neither.
func human(types byte) bool {
	return types&(1<<evKey|1<<evRel) != 0
}

// Watch signals on out whenever r delivers anything, at most once per
// throttle, until r fails (the device was unplugged or closed). What was
// read is thrown away. A signal already waiting on out is not doubled.
func Watch(r io.Reader, out chan<- struct{}, throttle time.Duration) {
	buf := make([]byte, 24*64) // whole input_events; the size does not matter
	for {
		n, err := r.Read(buf)
		if n > 0 {
			select {
			case out <- struct{}{}:
			default:
			}
			time.Sleep(throttle)
		}
		if err != nil {
			return
		}
	}
}
