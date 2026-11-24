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
	Theme    string   // a name from ui.ThemeNames()
	Scale    int      // pixels per logical pixel
	Rotate   Rotation // turn the picture on the framebuffer (a monitor mounted on its side)
	FPS      int      // animation frames per second; 0 = redraw on samples only
	Page     ui.Page
	Feeds    func() feeds.View // may be nil
	OnPage   func(ui.Page)     // told every page shown (socket/link demand); may be nil
	Intro    bool              // play the boot intro first (needs FPS > 0)
	Version  string            // for the intro's title card
	Attract  bool              // start with attract mode armed
	// Blank powers the monitor down after this long without input, until
	// the next input; 0 never (see CheckBlank). Keys on the console always
	// count; Activity, if not nil, signals input the console does not
	// carry (the mouse).
	Blank    time.Duration
	Activity <-chan struct{}
	// AfterOpen runs once the framebuffer and console are open, before
	// the first frame: where a root process drops privileges. tookVT says
	// whether restoring the console will need CAP_SYS_TTY_CONFIG.
	AfterOpen func(tookVT bool) error
}

const tween = 600 * time.Millisecond

// Run draws the dashboard on the framebuffer until q, Ctrl+C or a
// termination signal. On a VT it draws there; run as root from anywhere
// else (sudo, SSH) it takes a free VT and switches back when done. The console is restored on every exit path,
// including panics in this goroutine.
func Run(o Options) error {
	if o.Source == nil || o.Interval <= 0 || o.FPS < 0 || o.FPS > 60 || !CheckBlank(o.Blank) {
		return errors.New("gfx: invalid options")
	}
	sel, ok := ui.SelectTheme(o.Theme, time.Now())
	if !ok {
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
	fb.Rotate(o.Rotate)
	cols, rows, padX, padY, err := Grid(fb.W, fb.H, o.Scale)
	if err != nil {
		return err
	}
	con, err := OpenConsole()
	if err != nil {
		return err
	}
	defer con.Restore()
	if o.AfterOpen != nil {
		if err := o.AfterOpen(con.TookVT()); err != nil {
			return err
		}
	}

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

	page := o.Page
	want := o.OnPage
	if want == nil {
		want = func(ui.Page) {}
	}
	setPage := func(p ui.Page) {
		page = p
		want(p)
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
	bg := sel.Theme(start, o.FPS > 0).BG
	fb.Clear(rgb(bg))
	attract := ui.NewAttract(o.Attract)
	blank := NewBlanker(o.Blank, start)
	// The intro plays while the first samples come in; it needs animation.
	var intro *Intro
	if o.Intro && o.FPS > 0 {
		intro = &Intro{Sub: o.Version}
		if hn := cur.Host.Hostname; hn != "" {
			intro.Sub += " · " + hn
		}
	}
	skipIntro := func() {
		if intro != nil {
			intro, r, c = nil, NewRenderer(), nil
		}
	}

	// draw is called every animation tick. It re-lays out the dashboard
	// only when something it shows changed (a tween in progress, the
	// clock's second, a theme switch), and the renderer then redraws only
	// the tiles and stars that differ, so idle ticks cost almost nothing.
	draw := func() {
		if !active || blank.Off() {
			return
		}
		now := time.Now()
		th := sel.Theme(now, o.FPS > 0)
		if intro != nil {
			// Until a sample after the first lands there are no rates.
			el := now.Sub(start).Seconds()
			if !intro.Over(el, sampled.After(start)) {
				if spans := intro.Draw(cols*Tile, rows*Tile, th, el); len(spans) > 0 {
					fb.Blit(intro.Frame(), spans, o.Scale, padX, padY)
				}
				return
			}
			skipIntro()
		}
		k := min(float64(now.Sub(sampled))/float64(tween), 1)
		fv := feedView()
		// The news page types text continuously, so it lays out every
		// frame; tile diffing keeps that to the characters that appeared.
		as := attract.State()
		if c == nil || k < 1 || page == ui.News || c.sec != now.Unix() || c.theme != th || c.page != page || c.feeds != fv.Version || c.attract != as {
			snap := sys.Lerp(prev, cur, 1-(1-k)*(1-k)) // ease out
			v := ui.View{Page: page, Snap: snap, Feeds: fv, Now: now, Attract: as}
			reader.Apply(&v)
			c = &canvasCache{ui.Render(cols, rows, v, th, true), now.Unix(), th, page, fv.Version, as}
		}
		a := Anim{Sky: NewSky(now, fv)}
		if o.FPS > 0 {
			a.T = now.Sub(start).Seconds()
		}
		if th.BG != bg {
			// A cycle fade step: the margins around the frame follow.
			bg = th.BG
			fb.ClearMargins(rgb(bg), cols*Tile*o.Scale, rows*Tile*o.Scale, padX, padY)
		}
		if spans := r.Draw(c.canvas, th, a); len(spans) > 0 {
			fb.Blit(r.Frame(), spans, o.Scale, padX, padY)
		}
	}
	redrawAll := func() {
		r, c = NewRenderer(), nil
		if intro != nil {
			intro.Reset()
		}
		bg = sel.Theme(time.Now(), o.FPS > 0).BG
		fb.Clear(rgb(bg))
		draw()
	}
	// step moves attract mode's cycle on, on every tick, and puts the
	// monitor to sleep when the idle time has run out. Nothing cycles
	// behind a dark screen.
	step := func() {
		now := time.Now()
		if !active || blank.Off() {
			return
		}
		if blank.Step(now) {
			// A driver that cannot blank gets a black screen instead.
			if err := fb.Blank(true); err != nil {
				fb.Clear(0)
			}
			return
		}
		if p, ok := attract.Step(now, page); ok {
			setPage(p)
		}
	}
	// reblank puts a monitor back to sleep that the kernel lit while orb
	// had it blanked. Nobody touched anything (that would have come
	// through as input): a monitor going into deep sleep drops its link
	// some seconds after power-down, the driver takes that for a hotplug
	// and switches the output on again, and without this the screen would
	// stay lit, showing the last frame, for as long as nobody is there.
	// Called on every tick and sample: caught within a frame, the monitor
	// has not finished waking.
	reblank := func() {
		if active && blank.Off() && fb.Lit() {
			fb.Blank(true)
		}
	}
	// wake lights the monitor again; the pixels are redrawn from scratch.
	wake := func() {
		fb.Blank(false)
		redrawAll()
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
			reblank()
			if o.FPS == 0 {
				step()
				draw()
			}
		case <-o.Activity:
			if blank.Input(time.Now()) && active {
				wake()
			}
		case <-tick:
			reblank()
			step()
			draw()
		case k, ok := <-keys:
			if !ok {
				return errors.New("keyboard closed")
			}
			// A key that wakes the monitor does only that.
			if now := time.Now(); blank.Input(now) {
				wake()
				continue
			} else if blank.Waking(now) {
				continue
			}
			// Any key skips the intro and stops attract mode's cycle,
			// then does its usual job.
			skipIntro()
			attract.Key(time.Now())
			switch {
			case k.Code == KeyUp || k.Code == KeyLeft:
				reader.Skip(-1, time.Now())
				draw()
			case k.Code == KeyDown || k.Code == KeyRight:
				reader.Skip(1, time.Now())
				draw()
			case k.Rune == 'q' || k.Rune == 'Q' || k.Code == KeyInterrupt:
				return nil
			case k.Rune == 't' || k.Rune == 'T':
				sel = sel.Next(time.Now())
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
			case k.Rune == 'l' || k.Rune == 'L':
				setPage(ui.Links)
				draw()
			case k.Rune == ' ':
				reader.Toggle(time.Now())
				draw()
			case k.Rune == 'i' || k.Rune == 'I':
				attract.Toggle(time.Now())
				draw()
			default:
				draw() // the dashboard, if the key ended the intro
			}
		case s := <-sigs:
			switch s {
			case sigRelease:
				// The next VT must not inherit a dark monitor.
				if blank.Input(time.Now()) {
					fb.Blank(false)
				}
				active = false
				if err := con.Release(); err != nil {
					return fmt.Errorf("release VT: %w", err)
				}
			case sigAcquire:
				if err := con.Acquire(); err != nil {
					return fmt.Errorf("acquire VT: %w", err)
				}
				active = true
				blank.Input(time.Now()) // idle counts from here
				redrawAll()
			default:
				return nil
			}
		}
	}
}

type canvasCache struct {
	canvas  *canvas.Canvas
	sec     int64
	theme   *ui.Theme
	page    ui.Page
	feeds   uint64
	attract ui.AttractState
}
