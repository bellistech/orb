package gfx

import (
	"strings"
	"testing"
)

func TestScale2xRoundsADiagonal(t *testing.T) {
	// A one-pixel diagonal stays a smooth thin line, not a staircase of
	// 2x2 blocks (which plain enlarging would give: ##.., ##.., ..##, ..##).
	art := []string{
		"#.",
		".#",
	}
	got := scale2x(art)
	want := []string{
		"##..",
		"#.#.",
		".#.#",
		"..##",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestScalersKeepSolidsAndSizes(t *testing.T) {
	solid := []string{"###", "###"}
	for name, f := range map[string]func([]string) []string{"2x": scale2x, "3x": scale3x} {
		out := f(solid)
		k := len(out) / len(solid)
		if len(out[0]) != 3*k {
			t.Fatalf("%s: width %d", name, len(out[0]))
		}
		for _, l := range out {
			if strings.Trim(l, "#") != "" {
				t.Fatalf("%s: a solid block changed: %q", name, l)
			}
		}
	}
	for f := range dogFrames {
		up := henryArt(f)
		if len(up) != 2*dogH || len(up[0]) != 2*dogW {
			t.Fatalf("frame %d: %dx%d", f, len(up[0]), len(up))
		}
	}
}

func TestShadeTouchesOnlyTheBody(t *testing.T) {
	art := []string{"....", ".ww.", ".ww.", ".oo."}
	got := shade(art, "w", 'W', 'v', 0)
	want := []string{"....", ".WW.", ".vv.", ".oo."}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v want %v", got, want)
	}
}
