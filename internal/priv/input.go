package priv

import (
	"errors"
	"io"
)

// The input helper protocol (ADR-0005), helper -> main over the helper's
// stdout: "R\n" once the helper has dropped, then one "." whenever a
// keyboard or pointer did something, a few a second at most. That is all
// the main process ever learns about input devices; any other byte ends
// the helper.

// InputHelperFlag is the hidden argument that runs the input helper.
const InputHelperFlag = "-input-helper"

const (
	inputReady = "R\n"
	inputTick  = '.'
)

// pumpInput turns the helper's ticks into signals on out until the
// helper's output ends or misbehaves. A signal already waiting is not
// doubled.
func pumpInput(r io.Reader, out chan<- struct{}) error {
	buf := make([]byte, 64)
	for {
		n, err := r.Read(buf)
		for _, b := range buf[:n] {
			if b != inputTick {
				return errors.New("input helper: malformed output")
			}
		}
		if n > 0 {
			select {
			case out <- struct{}{}:
			default:
			}
		}
		if err != nil {
			return err
		}
	}
}
