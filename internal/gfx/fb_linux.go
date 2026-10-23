//go:build linux

package gfx

import (
	"encoding/binary"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ioctl numbers from <linux/fb.h>.
const (
	fbiogetVScreenInfo = 0x4600
	fbiogetFScreenInfo = 0x4602
)

// fbBitfield mirrors struct fb_bitfield.
type fbBitfield struct{ Offset, Length, MsbRight uint32 }

// fbVarInfo mirrors struct fb_var_screeninfo (160 bytes).
type fbVarInfo struct {
	XRes, YRes, XResVirtual, YResVirtual, XOffset, YOffset uint32
	BitsPerPixel, Grayscale                                uint32
	Red, Green, Blue, Transp                               fbBitfield
	NonStd, Activate, Height, Width, AccelFlags            uint32
	PixClock, LeftMargin, RightMargin, UpperMargin         uint32
	LowerMargin, HSyncLen, VSyncLen, Sync, VMode, Rotate   uint32
	Colorspace                                             uint32
	Reserved                                               [4]uint32
}

// fbFixInfo mirrors struct fb_fix_screeninfo on 64-bit (80 bytes).
type fbFixInfo struct {
	ID                             [16]byte
	SmemStart                      uint64
	SmemLen, Type, TypeAux, Visual uint32
	XPanStep, YPanStep, YWrapStep  uint16
	_                              uint16
	LineLength                     uint32
	_                              uint32
	MmioStart                      uint64
	MmioLen, Accel                 uint32
	Capabilities                   uint16
	Reserved                       [2]uint16
}

// FB is a mapped framebuffer device.
type FB struct {
	f      *os.File
	mem    []byte
	format pixelFormat
	W, H   int // visible pixels
	stride int // bytes per row
}

// OpenFB opens and maps a framebuffer device such as /dev/fb0.
func OpenFB(path string) (*FB, error) {
	f, err := openDevice(path, "a framebuffer device", isFB)
	if err != nil {
		return nil, fmt.Errorf("%w (graphics mode needs the video group)", err)
	}
	fb, err := mapFB(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return fb, nil
}

func mapFB(f *os.File) (*FB, error) {
	var v fbVarInfo
	var x fbFixInfo
	// The kernel fills these structs in place; unsafe is the only way to
	// hand it their address.
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, f.Fd(), fbiogetVScreenInfo, uintptr(unsafe.Pointer(&v))); e != 0 {
		return nil, fmt.Errorf("FBIOGET_VSCREENINFO: %w", e)
	}
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, f.Fd(), fbiogetFScreenInfo, uintptr(unsafe.Pointer(&x))); e != 0 {
		return nil, fmt.Errorf("FBIOGET_FSCREENINFO: %w", e)
	}
	layout, err := checkLayout(v, x)
	if err != nil {
		return nil, err
	}
	mem, err := unix.Mmap(int(f.Fd()), 0, layout.size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("mmap %d bytes: %w", layout.size, err)
	}
	return &FB{f: f, mem: mem, format: layout.format, W: int(v.XRes), H: int(v.YRes), stride: layout.stride}, nil
}

type fbLayout struct {
	format pixelFormat
	stride int
	size   int // bytes to map: through the last visible row
}

// checkLayout validates everything the kernel reported before any of it is
// used to index memory.
func checkLayout(v fbVarInfo, x fbFixInfo) (fbLayout, error) {
	const maxDim = 16384
	if v.XRes == 0 || v.YRes == 0 || v.XRes > maxDim || v.YRes > maxDim {
		return fbLayout{}, fmt.Errorf("unsupported resolution %dx%d", v.XRes, v.YRes)
	}
	if v.XOffset != 0 || v.YOffset != 0 {
		return fbLayout{}, fmt.Errorf("panned framebuffers (offset %d,%d) are not supported", v.XOffset, v.YOffset)
	}
	pf, err := newPixelFormat(v)
	if err != nil {
		return fbLayout{}, err
	}
	stride := int(x.LineLength)
	if stride < int(v.XRes)*pf.bytes || stride > maxDim*8 {
		return fbLayout{}, fmt.Errorf("line length %d too short for %d pixels at %d bytes", stride, v.XRes, pf.bytes)
	}
	size := stride * int(v.YRes)
	if x.SmemLen != 0 && size > int(x.SmemLen) {
		return fbLayout{}, fmt.Errorf("visible area %d bytes exceeds framebuffer memory %d", size, x.SmemLen)
	}
	return fbLayout{format: pf, stride: stride, size: size}, nil
}

// pixelFormat packs 8-bit RGB into the device's layout.
type pixelFormat struct {
	bytes            int
	rOff, gOff, bOff uint32
	rLen, gLen, bLen uint32
}

func newPixelFormat(v fbVarInfo) (pixelFormat, error) {
	if v.BitsPerPixel != 16 && v.BitsPerPixel != 32 {
		return pixelFormat{}, fmt.Errorf("unsupported depth %d bpp (want 16 or 32)", v.BitsPerPixel)
	}
	if v.Grayscale != 0 {
		return pixelFormat{}, fmt.Errorf("grayscale framebuffers are not supported")
	}
	pf := pixelFormat{
		bytes: int(v.BitsPerPixel / 8),
		rOff:  v.Red.Offset, gOff: v.Green.Offset, bOff: v.Blue.Offset,
		rLen: v.Red.Length, gLen: v.Green.Length, bLen: v.Blue.Length,
	}
	for _, b := range []fbBitfield{v.Red, v.Green, v.Blue} {
		if b.Length == 0 || b.Length > 8 || b.Offset+b.Length > v.BitsPerPixel || b.MsbRight != 0 {
			return pixelFormat{}, fmt.Errorf("unsupported color layout %+v", b)
		}
	}
	return pf, nil
}

func (p pixelFormat) pack(rgb uint32) uint32 {
	r, g, b := rgb>>16&0xFF, rgb>>8&0xFF, rgb&0xFF
	return r>>(8-p.rLen)<<p.rOff | g>>(8-p.gLen)<<p.gOff | b>>(8-p.bLen)<<p.bOff
}

// Close unmaps and closes the device.
func (fb *FB) Close() error {
	if fb == nil || fb.f == nil {
		return nil
	}
	err := unix.Munmap(fb.mem)
	if cerr := fb.f.Close(); err == nil {
		err = cerr
	}
	fb.f, fb.mem = nil, nil
	return err
}

// Clear fills the whole screen with one color.
func (fb *FB) Clear(rgb uint32) {
	px := fb.format.pack(rgb)
	row := make([]byte, fb.W*fb.format.bytes)
	putRow(row, fb.format.bytes, px, fb.W)
	for y := range fb.H {
		copy(fb.mem[y*fb.stride:], row)
	}
}

// ClearMargins fills the screen around a w x h picture at (padX, padY)
// with one color, leaving the picture alone.
func (fb *FB) ClearMargins(rgb uint32, w, h, padX, padY int) {
	px := fb.format.pack(rgb)
	bpp := fb.format.bytes
	padX, padY = max(0, min(padX, fb.W)), max(0, min(padY, fb.H))
	x1, y1 := min(padX+max(w, 0), fb.W), min(padY+max(h, 0), fb.H)
	row := make([]byte, fb.W*bpp)
	putRow(row, bpp, px, fb.W)
	for y := range fb.H {
		o := y * fb.stride
		if y < padY || y >= y1 {
			copy(fb.mem[o:o+fb.W*bpp], row)
			continue
		}
		copy(fb.mem[o:o+padX*bpp], row)
		copy(fb.mem[o+x1*bpp:o+fb.W*bpp], row)
	}
}

// Blit draws the given spans of f scaled up at (padX, padY); nil spans
// means the whole frame. It returns the number of spans written. Spans that
// would land outside the screen are skipped, never written.
func (fb *FB) Blit(f *Frame, spans []Span, scale, padX, padY int) int {
	if f == nil || scale < 1 || padX < 0 || padY < 0 ||
		padX+f.W*scale > fb.W || padY+f.H*scale > fb.H {
		return 0
	}
	if spans == nil {
		spans = make([]Span, f.H)
		for y := range spans {
			spans[y] = Span{Y: y, X0: 0, X1: f.W}
		}
	}
	bpp := fb.format.bytes
	row := make([]byte, f.W*scale*bpp)
	written := 0
	for _, sp := range spans {
		if sp.Y < 0 || sp.Y >= f.H || sp.X0 < 0 || sp.X1 > f.W || sp.X0 >= sp.X1 {
			continue
		}
		n := (sp.X1 - sp.X0) * scale * bpp
		for x, v := range f.Pix[sp.Y*f.W+sp.X0 : sp.Y*f.W+sp.X1] {
			px := fb.format.pack(v)
			for s := range scale {
				put(row[(x*scale+s)*bpp:], bpp, px)
			}
		}
		for s := range scale {
			off := (padY+sp.Y*scale+s)*fb.stride + (padX+sp.X0*scale)*bpp
			copy(fb.mem[off:off+n], row[:n])
		}
		written++
	}
	return written
}

func put(dst []byte, bpp int, px uint32) {
	if bpp == 4 {
		binary.NativeEndian.PutUint32(dst, px)
	} else {
		binary.NativeEndian.PutUint16(dst, uint16(px))
	}
}

func putRow(dst []byte, bpp int, px uint32, n int) {
	for i := range n {
		put(dst[i*bpp:], bpp, px)
	}
}
