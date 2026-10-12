// nocterm is a view-only host health dashboard for the terminal.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

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
	once := flag.String("once", "", "print one frame of size WxH (e.g. 215x45) and exit")
	demo := flag.Bool("demo", false, "show synthetic data instead of this host's")
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
	if ui.ThemeByName(*theme) == nil {
		fail("unknown theme %q (have: %s)", *theme, strings.Join(themeNames, ", "))
	}
	if *interval < 100*time.Millisecond {
		fail("interval must be at least 100ms")
	}
	if *once != "" {
		var w, h int
		if _, err := fmt.Sscanf(*once, "%dx%d", &w, &h); err != nil || w <= 0 || h <= 0 || w > 2000 || h > 1000 {
			fail("-once wants WxH, e.g. 215x45")
		}
		var snap *sys.Snapshot
		if *demo {
			snap = sys.Demo(time.Now())
		} else {
			s := sys.NewSampler()
			time.Sleep(*interval)
			snap = s.Sample()
		}
		fmt.Println(ui.Render(w, h, snap, ui.ThemeByName(*theme)).String())
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
