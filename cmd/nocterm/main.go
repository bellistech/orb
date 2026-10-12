// nocterm is a view-only host health dashboard for the terminal.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"nocterm/internal/gfx"
	"nocterm/internal/sys"
	"nocterm/internal/tui"
	"nocterm/internal/ui"
)

var version = "dev"

func main() {
	themeNames := make([]string, len(ui.Themes))
	for i, t := range ui.Themes {
		themeNames[i] = t.Name
	}
	theme := flag.String("theme", ui.Themes[0].Name, "color theme: "+strings.Join(themeNames, ", "))
	interval := flag.Duration("interval", time.Second, "sample interval")
	once := flag.String("once", "", "print one frame of WxH cells (pixels with -gfx, as PNG) and exit")
	gfxMode := flag.Bool("gfx", false, "draw pixels on the local console framebuffer instead of a terminal")
	scale := flag.Int("scale", 4, "graphics mode pixel scale, 1..8")
	fps := flag.Int("fps", 15, "graphics mode animation frames per second, 0 for none")
	device := flag.String("fb", "/dev/fb0", "graphics mode framebuffer device")
	demo := flag.Bool("demo", false, "show synthetic data instead of this host's")
	reset := flag.String("reset-console", "", "force a VT back to text mode (e.g. /dev/tty1, or \"-\" for this one) and exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: nocterm [flags]\n\nKeys: t next theme, q quit.\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Println("nocterm", version)
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
	if ui.ThemeByName(*theme) == nil {
		fail("unknown theme %q (have: %s)", *theme, strings.Join(themeNames, ", "))
	}
	if *interval < 100*time.Millisecond {
		fail("interval must be at least 100ms")
	}
	if *once != "" {
		var w, h int
		if _, err := fmt.Sscanf(*once, "%dx%d", &w, &h); err != nil || w <= 0 || h <= 0 || w > 16384 || h > 16384 {
			fail("-once wants WxH, e.g. 215x45 (or 3440x1440 with -gfx)")
		}
		var snap *sys.Snapshot
		if *demo {
			snap = sys.Demo(time.Now())
		} else {
			s := sys.NewSampler()
			time.Sleep(*interval)
			snap = s.Sample()
		}
		th := ui.ThemeByName(*theme)
		if *gfxMode {
			if fi, err := os.Stdout.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
				fail("-gfx -once writes a PNG; redirect stdout to a file")
			}
			cols, rows, _, _, err := gfx.Grid(w, h, *scale)
			if err != nil {
				fail("%v", err)
			}
			f := gfx.NewFrame(cols*gfx.Tile, rows*gfx.Tile)
			gfx.Rasterize(f, ui.Render(cols, rows, snap, th, true), th, gfx.Anim{})
			if err := gfx.WritePNG(os.Stdout, f, *scale); err != nil {
				fail("%v", err)
			}
			return
		}
		if w > 2000 || h > 1000 {
			fail("-once size too large for a terminal frame")
		}
		fmt.Println(ui.Render(w, h, snap, th, false).String())
		return
	}

	if *gfxMode {
		ti := 0
		for i, t := range ui.Themes {
			if t.Name == *theme {
				ti = i
			}
		}
		src := sys.NewSampler().Sample
		if *demo {
			src = func() *sys.Snapshot { return sys.Demo(time.Now()) }
		}
		if *fps < 0 || *fps > 60 {
			fail("-fps must be 0..60")
		}
		if err := gfx.Run(gfx.Options{Device: *device, Source: src, Interval: *interval, Theme: ti, Scale: *scale, FPS: *fps}); err != nil {
			fail("%v", err)
		}
		return
	}

	if _, err := tea.NewProgram(tui.New(*interval, *theme, *demo)).Run(); err != nil {
		fail("%v", err)
	}
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "nocterm: "+format+"\n", a...)
	os.Exit(2)
}
