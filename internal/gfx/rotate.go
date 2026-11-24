package gfx

import (
	"errors"
	"os"
	"strings"
)

// Rotation is how far graphics mode turns its picture on the framebuffer,
// in quarter turns clockwise: the numbering of the kernel's fbcon=rotate
// (0 normal, 1 90° clockwise, 2 upside down, 3 90° counter-clockwise). A
// monitor mounted turned 90° clockwise needs 3.
type Rotation int

// fbconRotate is where the kernel reports the console's rotation.
const fbconRotate = "/sys/class/graphics/fbcon/rotate"

// ParseRotation reads a -rotate value: "auto" (follow the console, see
// ConsoleRotation) or degrees clockwise, 0, 90, 180 or 270.
func ParseRotation(s string) (r Rotation, auto bool, err error) {
	switch s {
	case "auto":
		return 0, true, nil
	case "0":
		return 0, false, nil
	case "90":
		return 1, false, nil
	case "180":
		return 2, false, nil
	case "270":
		return 3, false, nil
	}
	return 0, false, errors.New("rotate must be auto, 0, 90, 180 or 270")
}

// ConsoleRotation is the text console's rotation (fbcon=rotate or
// /sys/class/graphics/fbcon/rotate_all), so graphics mode stands the same
// way up as the login it starts from. 0 when the kernel will not say.
func ConsoleRotation() Rotation { return consoleRotation(fbconRotate) }

func consoleRotation(path string) Rotation {
	b, err := os.ReadFile(path)
	if err != nil || len(b) > 8 {
		return 0
	}
	switch strings.TrimSpace(string(b)) {
	case "1":
		return 1
	case "2":
		return 2
	case "3":
		return 3
	}
	return 0
}

// physRect maps a w x h rectangle at (x, y) in the picture to the
// framebuffer rectangle it lands on, for a pw x ph framebuffer.
func (r Rotation) physRect(x, y, w, h, pw, ph int) (int, int, int, int) {
	switch r {
	case 1: // picture (x, y) -> (pw-1-y, x)
		return pw - y - h, x, h, w
	case 2: // -> (pw-1-x, ph-1-y)
		return pw - x - w, ph - y - h, w, h
	case 3: // -> (y, ph-1-x)
		return y, ph - x - w, h, w
	}
	return x, y, w, h
}
