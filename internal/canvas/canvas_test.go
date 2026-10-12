package canvas

import (
	"strings"
	"testing"
)

func TestSubClipsToParent(t *testing.T) {
	c := New(10, 4, Color{})
	r := c.Full().Sub(8, 2, 50, 50)
	if r.W != 2 || r.H != 2 {
		t.Fatalf("got %dx%d, want 2x2", r.W, r.H)
	}
	r.Text(0, 0, "abcdef", RGB(1, 2, 3)) // overflow is dropped
	r.Set(-1, -1, 'x', Color{})
	if got := strings.Split(c.Plain(), "\n")[2]; got != "        ab" {
		t.Fatalf("row 2 = %q", got)
	}
}

func TestStringEmitsStyleOnlyOnChange(t *testing.T) {
	c := New(4, 1, Color{})
	r := c.Full()
	r.Text(0, 0, "aa", RGB(255, 0, 0))
	r.Text(2, 0, "bb", RGB(0, 255, 0))
	s := c.String()
	if n := strings.Count(s, "38;2;"); n != 2 {
		t.Fatalf("fg sequences = %d, want 2: %q", n, s)
	}
	if !strings.HasSuffix(s, "\x1b[0m") {
		t.Fatalf("line not reset: %q", s)
	}
}

func TestHex(t *testing.T) {
	if got := Hex("#19d3ff"); got != RGB(0x19, 0xd3, 0xff) {
		t.Fatalf("got %+v", got)
	}
	for _, bad := range []string{"", "#fff", "#gggggg"} {
		if Hex(bad).Set {
			t.Fatalf("Hex(%q) should be unset", bad)
		}
	}
}

func TestDouble(t *testing.T) {
	c := New(5, 2, Color{})
	if x := c.Full().Double(0, 0, "ab", RGB(1, 1, 1), true); x != 4 {
		t.Fatalf("end column %d", x)
	}
	if c.CellAt(1, 1).Quad != 4 || c.CellAt(2, 0).Ch != 'b' || c.CellAt(2, 0).Quad != 1 {
		t.Fatalf("cells %+v %+v", c.CellAt(1, 1), c.CellAt(2, 0))
	}
	// Terminals see the rune once, in the top-left cell.
	if got := c.Plain(); got != "a b  \n     " {
		t.Fatalf("plain = %q", got)
	}
	// Plain writes clear the quadrant marker.
	c.Full().Set(0, 0, 'x', RGB(1, 1, 1))
	if c.CellAt(0, 0).Quad != 0 {
		t.Fatal("Set kept Quad")
	}
}

// Process names, interface names and feed text reach the canvas; none may
// smuggle terminal control sequences or invisible bidi overrides through.
func TestControlRunesNeutralized(t *testing.T) {
	c := New(12, 1, Color{})
	r := c.Full()
	r.Text(0, 0, "a\x1b[2J\x07\u202e\u009b", RGB(1, 1, 1)) // ESC, BEL, RLO, CSI (C1)
	r.Double(8, 0, "\x1b", RGB(1, 1, 1), false)
	r.Set(10, 0, '\n', RGB(1, 1, 1))
	if got := c.Plain(); got != "a?[2J???? ? " {
		t.Fatalf("plain = %q", got)
	}
	// The only escapes in the output are the canvas's own SGR sequences.
	out := c.String()
	for i := 0; i < len(out); i++ {
		if out[i] == 0x1b && (i+1 >= len(out) || out[i+1] != '[') {
			t.Fatalf("stray ESC at %d in %q", i, out)
		}
	}
	if strings.Contains(strings.ReplaceAll(out, "\x1b[", ""), "\x1b") {
		t.Fatalf("raw ESC in %q", out)
	}
}
