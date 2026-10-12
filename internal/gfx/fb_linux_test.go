//go:build linux

package gfx

import (
	"os"
	"testing"
)

func xrgb() fbVarInfo {
	return fbVarInfo{
		XRes: 3440, YRes: 1440, XResVirtual: 3440, YResVirtual: 1440, BitsPerPixel: 32,
		Red: fbBitfield{Offset: 16, Length: 8}, Green: fbBitfield{Offset: 8, Length: 8}, Blue: fbBitfield{Length: 8},
	}
}

func TestCheckLayout(t *testing.T) {
	good := fbFixInfo{LineLength: 13824, SmemLen: 13824 * 1440}
	l, err := checkLayout(xrgb(), good)
	if err != nil || l.stride != 13824 || l.size != 13824*1440 {
		t.Fatalf("got %+v, %v", l, err)
	}
	bad := map[string]func(*fbVarInfo, *fbFixInfo){
		"zero width":     func(v *fbVarInfo, _ *fbFixInfo) { v.XRes = 0 },
		"huge height":    func(v *fbVarInfo, _ *fbFixInfo) { v.YRes = 1 << 20 },
		"24 bpp":         func(v *fbVarInfo, _ *fbFixInfo) { v.BitsPerPixel = 24 },
		"short stride":   func(_ *fbVarInfo, x *fbFixInfo) { x.LineLength = 3440 },
		"small memory":   func(_ *fbVarInfo, x *fbFixInfo) { x.SmemLen = 1024 },
		"panned":         func(v *fbVarInfo, _ *fbFixInfo) { v.YOffset = 1440 },
		"field overflow": func(v *fbVarInfo, _ *fbFixInfo) { v.Red.Offset = 30 },
		"zero length":    func(v *fbVarInfo, _ *fbFixInfo) { v.Green.Length = 0 },
		"grayscale":      func(v *fbVarInfo, _ *fbFixInfo) { v.Grayscale = 1 },
		"msb right":      func(v *fbVarInfo, _ *fbFixInfo) { v.Blue.MsbRight = 1 },
	}
	for name, mutate := range bad {
		v, x := xrgb(), good
		mutate(&v, &x)
		if _, err := checkLayout(v, x); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestPackRGB565(t *testing.T) {
	v := fbVarInfo{XRes: 1, YRes: 1, BitsPerPixel: 16,
		Red: fbBitfield{Offset: 11, Length: 5}, Green: fbBitfield{Offset: 5, Length: 6}, Blue: fbBitfield{Length: 5}}
	pf, err := newPixelFormat(v)
	if err != nil {
		t.Fatal(err)
	}
	if got := pf.pack(0xFFFFFF); got != 0xFFFF {
		t.Fatalf("white = %#x", got)
	}
	if got := pf.pack(0xFF0000); got != 0xF800 {
		t.Fatalf("red = %#x", got)
	}
}

func TestBlitScalesAndSkipsUnchangedRows(t *testing.T) {
	pf, _ := newPixelFormat(xrgb())
	fb := &FB{format: pf, W: 8, H: 6, stride: 8*4 + 8} // padded stride
	fb.mem = make([]byte, fb.stride*fb.H)
	f := NewFrame(2, 2)
	f.Pix = []uint32{0x112233, 0x445566, 0x778899, 0xAABBCC}
	if n := fb.Blit(f, nil, 2, 1, 1); n != 2 {
		t.Fatalf("wrote %d spans, want 2", n)
	}
	px := func(x, y int) uint32 {
		o := y*fb.stride + x*4
		return uint32(fb.mem[o]) | uint32(fb.mem[o+1])<<8 | uint32(fb.mem[o+2])<<16
	}
	for _, c := range []struct {
		x, y int
		want uint32
	}{
		{1, 1, 0x112233}, {2, 2, 0x112233}, {3, 1, 0x445566}, {4, 2, 0x445566}, {1, 3, 0x778899}, {4, 4, 0xAABBCC}, {0, 0, 0}, {5, 1, 0},
	} {
		if got := px(c.x, c.y); got != c.want {
			t.Errorf("pixel %d,%d = %#06x, want %#06x", c.x, c.y, got, c.want)
		}
	}
	// A span writes only its own columns.
	f.Pix[3] = 0
	if n := fb.Blit(f, []Span{{Y: 1, X0: 1, X1: 2}}, 2, 1, 1); n != 1 {
		t.Fatalf("span blit wrote %d, want 1", n)
	}
	if got := px(3, 3); got != 0 {
		t.Errorf("span pixel = %#06x, want 0", got)
	}
	if got := px(1, 3); got != 0x778899 {
		t.Errorf("pixel outside span changed: %#06x", got)
	}
	// Hostile spans are ignored.
	if n := fb.Blit(f, []Span{{Y: -1, X0: 0, X1: 1}, {Y: 0, X0: 1, X1: 9}, {Y: 5}}, 2, 1, 1); n != 0 {
		t.Fatalf("bad spans wrote %d", n)
	}
	// Out-of-bounds placement is refused rather than written.
	if n := fb.Blit(f, nil, 4, 1, 1); n != 0 {
		t.Fatalf("oversized blit wrote %d rows", n)
	}
}

// Reads the real device's geometry when present; never writes to it.
func TestOpenRealFramebuffer(t *testing.T) {
	if _, err := os.Stat("/dev/fb0"); err != nil {
		t.Skip("no /dev/fb0")
	}
	fb, err := OpenFB("/dev/fb0")
	if err != nil {
		t.Skipf("cannot open: %v", err)
	}
	defer fb.Close()
	if fb.W <= 0 || fb.H <= 0 || fb.stride < fb.W*fb.format.bytes {
		t.Fatalf("bad geometry %dx%d stride %d", fb.W, fb.H, fb.stride)
	}
	t.Logf("fb0: %dx%d, stride %d, %d bytes/px", fb.W, fb.H, fb.stride, fb.format.bytes)
}
