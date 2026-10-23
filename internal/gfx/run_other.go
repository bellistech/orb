//go:build !linux

package gfx

import (
	"errors"
	"time"

	"orb/internal/feeds"
	"orb/internal/sys"
	"orb/internal/ui"
)

// Options configures a graphics-mode run.
type Options struct {
	Device    string
	Source    func() *sys.Snapshot
	Interval  time.Duration
	Theme     string
	Scale     int
	FPS       int
	Page      ui.Page
	Feeds     func() feeds.View
	OnPage    func(ui.Page)
	Intro     bool
	Version   string
	Attract   bool
	AfterOpen func(tookVT bool) error
}

// Run is only available on Linux.
func Run(Options) error { return errors.New("graphics mode needs the Linux framebuffer") }

// ResetConsole is only available on Linux.
func ResetConsole(string) error { return errors.New("console reset needs Linux") }
