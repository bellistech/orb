package gfx

import (
	"testing"
	"time"

	"orb/internal/feeds"
	"orb/internal/sys"
	"orb/internal/ui"
)

func TestFontArtWellFormed(t *testing.T) {
	for r, art := range fontArt {
		if _, err := parseGlyph(art); err != nil {
			t.Errorf("glyph %q: %v", r, err)
		}
	}
}

func TestParseGlyphRejectsBadArt(t *testing.T) {
	for _, bad := range []string{
		"",
		"#####", // too few rows
		"..... ..... ..... ..... ..... ..... ..... ..... .....", // 9 rows
		"#### ..... ..... ..... ..... ..... .....",              // short row
		"..x.. ..... ..... ..... ..... ..... .....",             // bad char
	} {
		if _, err := parseGlyph(bad); err == nil {
			t.Errorf("parseGlyph(%q) accepted bad art", bad)
		}
	}
}

func TestPrintableASCIIComplete(t *testing.T) {
	for r := rune(32); r < 127; r++ {
		if !Supported(r) {
			t.Errorf("missing glyph for %q", r)
		}
	}
}

// Every rune the dashboard draws must render in graphics mode.
func TestEveryDashboardRuneSupported(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	demo := sys.Demo(now)
	for _, th := range ui.Themes {
		for _, v := range []ui.View{{Page: ui.Dash, Snap: demo}, {Page: ui.News, Snap: demo, Feeds: feeds.Demo(now), Now: now}, {Page: ui.News, Snap: demo}, {Page: ui.Net, Snap: demo}, {Page: ui.Links, Snap: demo}} {
			for _, size := range [][2]int{{107, 45}, {143, 60}, {215, 45}, {80, 60}} {
				c := ui.Render(size[0], size[1], v, th, true)
				for y := range c.H {
					for x := range c.W {
						if r := c.CellAt(x, y).Ch; !Supported(r) {
							t.Fatalf("%s %v: rune %q (%U) at %d,%d has no glyph", th.Name, size, r, r, x, y)
						}
					}
				}
			}
		}
	}
}
