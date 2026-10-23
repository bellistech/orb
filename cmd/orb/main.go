// orb is a view-only host health dashboard for the terminal and the
// Linux console.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
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
	// The LLDP listener (ADR-0003), likewise re-executed by a root orb.
	if len(os.Args) == 2 && os.Args[1] == priv.LLDPHelperFlag {
		if err := priv.RunLLDPHelper(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "orb lldp helper:", err)
			os.Exit(3)
		}
		return
	}

	themeNames := ui.ThemeNames()
	cfgPath := flag.String("config", config.DefaultPath(), "config file")
	theme := flag.String("theme", ui.DefaultTheme, "color theme: "+strings.Join(themeNames, ", ")+" ("+ui.Cycle+" loops through the rest)")
	page := flag.String("page", "dash", "starting page: dash, news, net or links")
	interval := flag.Duration("interval", time.Second, "sample interval")
	once := flag.String("once", "", "print one frame of WxH cells (pixels with -gfx, as PNG) and exit")
	gfxMode := flag.Bool("gfx", false, "draw pixels on the local console framebuffer instead of a terminal")
	scale := flag.Int("scale", 3, "graphics mode pixel scale, 1..8")
	fps := flag.Int("fps", 15, "graphics mode animation frames per second, 0 for none")
	device := flag.String("fb", "/dev/fb0", "graphics mode framebuffer device")
	intro := flag.Bool("intro", true, "graphics mode: play the boot intro on start (any key skips)")
	attract := flag.Bool("attract", false, "start with attract mode armed: cycle the pages after 30s without a key")
	newsURL := flag.String("news-url", "", "news JSON URL (better kept in the config file)")
	weatherURL := flag.String("weather-url", "", "weather JSON URL")
	demo := flag.Bool("demo", false, "show synthetic data instead of this host's")
	reset := flag.String("reset-console", "", "force a VT back to text mode (e.g. /dev/tty1, or \"-\" for this one) and exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: orb [flags]\n\nKeys: d dashboard, w news & weather, n network, l links (L2/L3), space pause/resume the reader,\n      up/left previous story, down/right next story, t next theme, i attract mode, q quit.\n\nFlags:\n")
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
	load := config.Load
	if os.Geteuid() == 0 {
		// Still root: read only a regular file owned by the user it belongs
		// to (sudo's invoking user, else root), never what a symlink names.
		owner := 0
		if t := priv.TargetFromEnv(os.Getenv); os.Getenv("SUDO_UID") == strconv.Itoa(t.UID) {
			owner = t.UID
		}
		load = func(path string) (config.Config, string, error) { return config.LoadOwned(path, owner) }
	}
	cfg, warn, err := load(*cfgPath)
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
	if !set["intro"] && cfg.NoIntro {
		*intro = false
	}
	if !set["attract"] && cfg.Attract {
		*attract = true
	}
	if !set["news-url"] {
		*newsURL = cfg.NewsURL
	}
	if !set["weather-url"] {
		*weatherURL = cfg.WeatherURL
	}

	sel, ok := ui.SelectTheme(*theme, time.Now())
	if !ok {
		fail("unknown theme %q (have: %s)", *theme, strings.Join(themeNames, ", "))
	}
	startPage := ui.Dash
	switch *page {
	case "dash":
	case "news":
		startPage = ui.News
	case "net":
		startPage = ui.Net
	case "links":
		startPage = ui.Links
	default:
		fail("-page must be dash, news, net or links")
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
	src := sampler.Sample
	// Sockets and links are read only while their page is up.
	onPage := func(p ui.Page) {
		sampler.WantSockets.Store(p == ui.Net)
		sampler.WantLinks.Store(p == ui.Links)
	}
	feedView := func() feeds.View { return feeds.View{} }
	startFeeds := func() {}
	if *demo {
		onPage = func(ui.Page) {}
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
		if t, stop, err := priv.StartLLDPHelper(); err != nil {
			fmt.Fprintf(os.Stderr, "orb: warning: %v; the links page will hear no LLDP neighbors\n", err)
		} else {
			sampler.LLDP = t.Neighbors
			defer stop()
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
		onPage(startPage)
		runOnce(*once, *gfxMode, *scale, ui.View{Page: startPage, Snap: onceSnap(sampler, *demo, *interval), Feeds: onceFeeds(feedView, startPage), Now: time.Now()}, sel.Theme(time.Now(), false))
		return
	}

	if *gfxMode {
		err := gfx.Run(gfx.Options{Device: *device, Source: src, Interval: *interval, Theme: sel.Name(),
			Scale: *scale, FPS: *fps, Page: startPage, Feeds: feedView, OnPage: onPage,
			Intro: *intro, Version: version, Attract: *attract,
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

	if _, err := tea.NewProgram(tui.New(*interval, sel.Name(), startPage, src, feedView, onPage, *attract)).Run(); err != nil {
		fail("%v", err)
	}
}

func onceSnap(s *sys.Sampler, demo bool, interval time.Duration) *sys.Snapshot {
	if demo {
		return sys.Demo(time.Now())
	}
	if s.WantLinks.Load() {
		s.Sample() // link rates need a sample with links on before this one
		// LLDP neighbors announce every 30 s: give one frame a chance.
		for deadline := time.Now().Add(35 * time.Second); s.LLDP != nil && time.Now().Before(deadline); time.Sleep(250 * time.Millisecond) {
			if ns, on := s.LLDP(); !on || len(ns) > 0 {
				break
			}
		}
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
