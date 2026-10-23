// orb is a view-only host health dashboard for the terminal and the
// Linux console.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"orb/internal/config"
	"orb/internal/feeds"
	"orb/internal/gfx"
	"orb/internal/priv"
	"orb/internal/sys"
	"orb/internal/tui"
	"orb/internal/ui"
)

var version = "dev"

func main() {
	// The privileged socket-owner helper is this binary re-executed by a
	// root orb; it must not parse flags, read config or touch anything else.
	if len(os.Args) == 2 && os.Args[1] == priv.HelperFlag {
		if err := priv.RunHelper(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "orb socket helper:", err)
			os.Exit(3)
		}
		return
	}

	themeNames := make([]string, len(ui.Themes))
	for i, t := range ui.Themes {
		themeNames[i] = t.Name
	}
	cfgPath := flag.String("config", config.DefaultPath(), "config file")
	theme := flag.String("theme", ui.DefaultTheme, "color theme: "+strings.Join(themeNames, ", "))
	page := flag.String("page", "dash", "starting page: dash, news or net")
	interval := flag.Duration("interval", time.Second, "sample interval")
	once := flag.String("once", "", "print one frame of WxH cells (pixels with -gfx, as PNG) and exit")
	gfxMode := flag.Bool("gfx", false, "draw pixels on the local console framebuffer instead of a terminal")
	scale := flag.Int("scale", 3, "graphics mode pixel scale, 1..8")
	fps := flag.Int("fps", 15, "graphics mode animation frames per second, 0 for none")
	device := flag.String("fb", "/dev/fb0", "graphics mode framebuffer device")
	newsURL := flag.String("news-url", "", "news JSON URL (better kept in the config file)")
	weatherURL := flag.String("weather-url", "", "weather JSON URL")
	demo := flag.Bool("demo", false, "show synthetic data instead of this host's")
	reset := flag.String("reset-console", "", "force a VT back to text mode (e.g. /dev/tty1, or \"-\" for this one) and exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: orb [flags]\n\nKeys: d dashboard, w news & weather, n network, space pause/resume the reader,\n      up/left previous story, down/right next story, t next theme, q quit.\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Println("orb", version)
		return
	}
	if *reset != "" {
		tty := *reset
		if tty == "-" {
			tty = ""
		}
		if err := gfx.ResetConsole(tty); err != nil {
			fail("%v", err)
		}
		return
	}

	// The config file supplies defaults; flags given on the command line win.
	cfg, warn, err := config.Load(*cfgPath)
	if err != nil {
		fail("%v", err)
	}
	if warn != "" {
		fmt.Fprintln(os.Stderr, "orb: warning:", warn)
	}
	set := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if !set["theme"] && cfg.Theme != "" {
		*theme = cfg.Theme
	}
	if !set["page"] && cfg.Page != "" {
		*page = cfg.Page
	}
	if !set["interval"] && cfg.Interval != 0 {
		*interval = cfg.Interval
	}
	if !set["scale"] && cfg.Scale != 0 {
		*scale = cfg.Scale
	}
	if !set["fps"] && cfg.HasFPS {
		*fps = cfg.FPS
	}
	if !set["news-url"] {
		*newsURL = cfg.NewsURL
	}
	if !set["weather-url"] {
		*weatherURL = cfg.WeatherURL
	}

	th := ui.ThemeByName(*theme)
	if th == nil {
		fail("unknown theme %q (have: %s)", *theme, strings.Join(themeNames, ", "))
	}
	startPage := ui.Dash
	switch *page {
	case "dash":
	case "news":
		startPage = ui.News
	case "net":
		startPage = ui.Net
	default:
		fail("-page must be dash, news or net")
	}
	if *interval < 100*time.Millisecond {
		fail("interval must be at least 100ms")
	}
	if *fps < 0 || *fps > 60 {
		fail("-fps must be 0..60")
	}
	for _, u := range []string{*newsURL, *weatherURL} {
		if u != "" {
			if err := feeds.CheckURL(u); err != nil {
				fail("%v", err)
			}
		}
	}

	sampler := sys.NewSampler()
	src, wantSockets := sampler.Sample, sampler.WantSockets.Store
	feedView := func() feeds.View { return feeds.View{} }
	startFeeds := func() {}
	if *demo {
		wantSockets = func(bool) {}
		src = func() *sys.Snapshot { return sys.Demo(time.Now()) }
		dv := feeds.Demo(time.Now()) // built once: the reader caches by identity
		feedView = func() feeds.View { return dv }
	} else if *newsURL != "" || *weatherURL != "" {
		store := &feeds.Store{}
		feedView = store.View
		startFeeds = func() {
			store.Start(context.Background(), feeds.NewClient("orb/"+version, 30*time.Second), *newsURL, *weatherURL)
		}
	}

	// As root: a separate helper keeps only what reading every process's
	// sockets needs, and this process drops to the invoking user before
	// any feed is fetched or parsed. Failure to drop is fatal.
	drop := func(bool) error { return nil }
	if os.Geteuid() == 0 && !*demo {
		if h, err := priv.StartHelper(); err != nil {
			fmt.Fprintf(os.Stderr, "orb: warning: %v; the network page will not see other users' processes\n", err)
		} else {
			sampler.Owners, sampler.AllOwners = h.Owners, true
			defer h.Close()
		}
		target := priv.TargetFromEnv(os.Getenv)
		drop = func(tookVT bool) error {
			var keep []priv.Cap
			if tookVT {
				keep = append(keep, priv.CapSysTTYConfig) // to give the console back
			}
			if err := priv.Drop(target, keep...); err != nil {
				return fmt.Errorf("dropping root: %w", err)
			}
			return nil
		}
	}
	// Terminal and one-shot modes have nothing privileged to open: drop now.
	if !*gfxMode || *once != "" {
		if err := drop(false); err != nil {
			fail("%v", err)
		}
		startFeeds()
	}

	if *once != "" {
		wantSockets(startPage == ui.Net)
		runOnce(*once, *gfxMode, *scale, ui.View{Page: startPage, Snap: onceSnap(sampler, *demo, *interval), Feeds: onceFeeds(feedView, startPage), Now: time.Now()}, th)
		return
	}

	if *gfxMode {
		ti := 0
		for i, t := range ui.Themes {
			if t == th {
				ti = i
			}
		}
		err := gfx.Run(gfx.Options{Device: *device, Source: src, Interval: *interval, Theme: ti,
			Scale: *scale, FPS: *fps, Page: startPage, Feeds: feedView, Sockets: wantSockets,
			AfterOpen: func(tookVT bool) error {
				if err := drop(tookVT); err != nil {
					return err
				}
				startFeeds()
				return nil
			}})
		if err != nil {
			fail("%v", err)
		}
		return
	}

	if _, err := tea.NewProgram(tui.New(*interval, th.Name, startPage, src, feedView, wantSockets)).Run(); err != nil {
		fail("%v", err)
	}
}

func onceSnap(s *sys.Sampler, demo bool, interval time.Duration) *sys.Snapshot {
	if demo {
		return sys.Demo(time.Now())
	}
	time.Sleep(interval)
	return s.Sample()
}

// onceFeeds waits briefly for the first fetch when printing a news frame.
func onceFeeds(view func() feeds.View, p ui.Page) feeds.View {
	v := view()
	for deadline := time.Now().Add(20 * time.Second); p == ui.News && (v.NewsOn || v.WeatherOn) &&
		(v.NewsOn && v.News == nil && v.NewsErr == "" || v.WeatherOn && v.Weather == nil && v.WeatherErr == "") &&
		time.Now().Before(deadline); v = view() {
		time.Sleep(100 * time.Millisecond)
	}
	return v
}

func runOnce(size string, gfxMode bool, scale int, v ui.View, th *ui.Theme) {
	var w, h int
	if _, err := fmt.Sscanf(size, "%dx%d", &w, &h); err != nil || w <= 0 || h <= 0 || w > 16384 || h > 16384 {
		fail("-once wants WxH, e.g. 215x45 (or 3440x1440 with -gfx)")
	}
	if !gfxMode {
		if w > 2000 || h > 1000 {
			fail("-once size too large for a terminal frame")
		}
		fmt.Println(ui.Render(w, h, v, th, false).String())
		return
	}
	if fi, err := os.Stdout.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
		fail("-gfx -once writes a PNG; redirect stdout to a file")
	}
	cols, rows, _, _, err := gfx.Grid(w, h, scale)
	if err != nil {
		fail("%v", err)
	}
	f := gfx.NewFrame(cols*gfx.Tile, rows*gfx.Tile)
	gfx.Rasterize(f, ui.Render(cols, rows, v, th, true), th, gfx.Anim{})
	if err := gfx.WritePNG(os.Stdout, f, scale); err != nil {
		fail("%v", err)
	}
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "orb: "+format+"\n", a...)
	os.Exit(2)
}
