package gfx

import (
	"fmt"
	"image"
	"image/png"
	"io"
)

// MaxScale bounds the pixel scale factor.
const MaxScale = 8

// Grid returns how many tiles fit a screen of pw x ph physical pixels at
// scale, and the leftover border to center the frame.
func Grid(pw, ph, scale int) (cols, rows, padX, padY int, err error) {
	if scale < 1 || scale > MaxScale {
		return 0, 0, 0, 0, fmt.Errorf("scale %d outside 1..%d", scale, MaxScale)
	}
	if pw <= 0 || ph <= 0 {
		return 0, 0, 0, 0, fmt.Errorf("bad screen size %dx%d", pw, ph)
	}
	cols, rows = pw/(Tile*scale), ph/(Tile*scale)
	if cols < 40 || rows < 12 {
		return 0, 0, 0, 0, fmt.Errorf("%dx%d at scale %d gives %dx%d tiles; need at least 40x12", pw, ph, scale, cols, rows)
	}
	return cols, rows, (pw - cols*Tile*scale) / 2, (ph - rows*Tile*scale) / 2, nil
}

// WritePNG encodes f scaled up by an integer factor with hard pixel edges.
func WritePNG(w io.Writer, f *Frame, scale int) error {
	if f == nil || scale < 1 || scale > MaxScale {
		return fmt.Errorf("bad frame or scale")
	}
	img := image.NewRGBA(image.Rect(0, 0, f.W*scale, f.H*scale))
	for y := range img.Rect.Dy() {
		for x := range img.Rect.Dx() {
			v := f.Pix[(y/scale)*f.W+x/scale]
			i := img.PixOffset(x, y)
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = uint8(v>>16), uint8(v>>8), uint8(v), 0xFF
		}
	}
	return png.Encode(w, img)
}
