//go:build linux

package gfx

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nocterm/internal/sys"
	"nocterm/internal/ui"
)

// Options configures a graphics-mode run.
type Options struct {
	Device   string               // framebuffer, e.g. /dev/fb0
	Source   func() *sys.Snapshot // called once per Interval
	Interval time.Duration
	Theme    int // index into ui.Themes
	Scale    int // pixels per logical pixel
	FPS      int // animation frames per second; 0 = redraw on samples only
}

const tween = 600 * time.Millisecond

// Run draws the dashboard on the framebuffer until q, Ctrl+C or a
// termination signal. The console is restored on every exit path,
// including panics in this goroutine.
func Run(o Options) error {
	if o.Source == nil || o.Interval <= 0 || o.FPS < 0 || o.FPS > 60 || o.Theme < 0 || o.Theme >= len(ui.Themes) {
		return errors.New("gfx: invalid options")
	}
	// Signals first: once the VT is in process mode, an unhandled
	// SIGUSR1 would kill us with the console stuck in graphics.
	sigs := make(chan os.Signal, 8)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, sigRelease, sigAcquire)
	defer signal.Stop(sigs)

	fb, err := OpenFB(o.Device)
	if err != nil {
		return err
	}
	defer fb.Close()
	cols, rows, padX, padY, err := Grid(fb.W, fb.H, o.Scale)
	if err != nil {
		return err
	}
	con, err := OpenConsole()
	if err != nil {
		return err
	}
	defer con.Restore()

	keys := make(chan byte, 16)
	go func() {
		buf := make([]byte, 16)
		for {
			n, err := con.Read(buf)
			if err != nil {
				close(keys)
				return
			}
			for _, b := range buf[:n] {
				select {
				case keys <- b:
				default: // drop keys nobody is reading
				}
			}
		}
	}()

	theme := o.Theme
	start := time.Now()
	cur := o.Source()
	prev, sampled := cur, start
	frame, last := NewFrame(cols*Tile, rows*Tile), (*Frame)(nil)
	active := true
	fb.Clear(rgb(ui.Themes[theme].BG))

	draw := func() {
		if !active {
			return
		}
		th := ui.Themes[theme]
		k := min(float64(time.Since(sampled))/float64(tween), 1)
		snap := sys.Lerp(prev, cur, 1-(1-k)*(1-k)) // ease out
		a := Anim{}
		if o.FPS > 0 {
			a.T = time.Since(start).Seconds()
		}
		Rasterize(frame, ui.Render(cols, rows, snap, th, true), th, a)
		fb.Blit(frame, last, o.Scale, padX, padY)
		if last == nil {
			last = NewFrame(frame.W, frame.H)
		}
		copy(last.Pix, frame.Pix)
	}

	sample := time.NewTicker(o.Interval)
	defer sample.Stop()
	var tick <-chan time.Time
	if o.FPS > 0 {
		t := time.NewTicker(time.Second / time.Duration(o.FPS))
		defer t.Stop()
		tick = t.C
	}
	draw()
	for {
		select {
		case <-sample.C:
			prev, cur, sampled = cur, o.Source(), time.Now()
			if o.FPS == 0 {
				draw()
			}
		case <-tick:
			draw()
		case b, ok := <-keys:
			if !ok {
				return errors.New("keyboard closed")
			}
			switch b {
			case 'q', 'Q', 0x1b:
				return nil
			case 't', 'T':
				theme = (theme + 1) % len(ui.Themes)
				fb.Clear(rgb(ui.Themes[theme].BG))
				last = nil
				draw()
			}
		case s := <-sigs:
			switch s {
			case sigRelease:
				active = false
				if err := con.Release(); err != nil {
					return fmt.Errorf("release VT: %w", err)
				}
			case sigAcquire:
				if err := con.Acquire(); err != nil {
					return fmt.Errorf("acquire VT: %w", err)
				}
				active, last = true, nil
				fb.Clear(rgb(ui.Themes[theme].BG))
				draw()
			default:
				return nil
			}
		}
	}
}
