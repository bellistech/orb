//go:build linux

package gfx

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"orb/internal/canvas"
	"orb/internal/feeds"
	"orb/internal/sys"
	"orb/internal/ui"
)

// Options configures a graphics-mode run.
type Options struct {
	Device   string               // framebuffer, e.g. /dev/fb0
	Source   func() *sys.Snapshot // called once per Interval
	Interval time.Duration
	Theme    int // index into ui.Themes
	Scale    int // pixels per logical pixel
	FPS      int // animation frames per second; 0 = redraw on samples only
	Page     ui.Page
	Feeds    func() feeds.View // may be nil
	Sockets  func(bool)        // asks the source for sockets; may be nil
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

	keys := make(chan Key, 16)
	go func() {
		buf := make([]byte, 64)
		for {
			n, err := con.Read(buf)
			if err != nil {
				close(keys)
				return
			}
			for _, k := range ParseKeys(buf[:n]) {
				select {
				case keys <- k:
				default: // drop keys nobody is reading
				}
			}
		}
	}()

	theme, page := o.Theme, o.Page
	want := o.Sockets
	if want == nil {
		want = func(bool) {}
	}
	setPage := func(p ui.Page) {
		page = p
		want(p == ui.Net)
	}
	setPage(page)
	feedView := o.Feeds
	if feedView == nil {
		feedView = func() feeds.View { return feeds.View{} }
	}
	start := time.Now()
	var reader ui.ReaderClock
	cur := o.Source()
	prev, sampled := cur, start
	r := NewRenderer()
	var c *canvasCache
	active := true
	fb.Clear(rgb(ui.Themes[theme].BG))

	// draw is called every animation tick. It re-lays out the dashboard
	// only when something it shows changed (a tween in progress, the
	// clock's second, a theme switch), and the renderer then redraws only
	// the tiles and stars that differ, so idle ticks cost almost nothing.
	draw := func() {
		if !active {
			return
		}
		th := ui.Themes[theme]
		now := time.Now()
		k := min(float64(now.Sub(sampled))/float64(tween), 1)
		fv := feedView()
		// The news page types text continuously, so it lays out every
		// frame; tile diffing keeps that to the characters that appeared.
		if c == nil || k < 1 || page == ui.News || c.sec != now.Unix() || c.theme != theme || c.page != page || c.feeds != fv.Version {
			snap := sys.Lerp(prev, cur, 1-(1-k)*(1-k)) // ease out
			v := ui.View{Page: page, Snap: snap, Feeds: fv, Now: now}
			reader.Apply(&v)
			c = &canvasCache{ui.Render(cols, rows, v, th, true), now.Unix(), theme, page, fv.Version}
		}
		a := Anim{}
		if o.FPS > 0 {
			a.T = now.Sub(start).Seconds()
		}
		if spans := r.Draw(c.canvas, th, a); len(spans) > 0 {
			fb.Blit(r.Frame(), spans, o.Scale, padX, padY)
		}
	}
	redrawAll := func() {
		r, c = NewRenderer(), nil
		fb.Clear(rgb(ui.Themes[theme].BG))
		draw()
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
		case k, ok := <-keys:
			if !ok {
				return errors.New("keyboard closed")
			}
			switch {
			case k.Code == KeyUp || k.Code == KeyLeft:
				reader.Skip(-1, time.Now())
				draw()
			case k.Code == KeyDown || k.Code == KeyRight:
				reader.Skip(1, time.Now())
				draw()
			case k.Rune == 'q' || k.Rune == 'Q':
				return nil
			case k.Rune == 't' || k.Rune == 'T':
				theme = (theme + 1) % len(ui.Themes)
				redrawAll()
			case k.Rune == 'd' || k.Rune == 'D':
				setPage(ui.Dash)
				draw()
			case k.Rune == 'w' || k.Rune == 'W':
				setPage(ui.News)
				draw()
			case k.Rune == 'n' || k.Rune == 'N':
				setPage(ui.Net)
				draw()
			case k.Rune == ' ':
				reader.Toggle(time.Now())
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
				active = true
				redrawAll()
			default:
				return nil
			}
		}
	}
}

type canvasCache struct {
	canvas *canvas.Canvas
	sec    int64
	theme  int
	page   ui.Page
	feeds  uint64
}
