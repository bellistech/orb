//go:build !linux

package gfx

import (
	"errors"
	"time"

	"nocterm/internal/sys"
)

// Options configures a graphics-mode run.
type Options struct {
	Device   string
	Source   func() *sys.Snapshot
	Interval time.Duration
	Theme    int
	Scale    int
	FPS      int
}

// Run is only available on Linux.
func Run(Options) error { return errors.New("graphics mode needs the Linux framebuffer") }

// ResetConsole is only available on Linux.
func ResetConsole(string) error { return errors.New("console reset needs Linux") }
