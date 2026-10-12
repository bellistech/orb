package gfx

// Key is one key press read from the console.
type Key struct {
	Code KeyCode // non-zero for special keys
	Rune byte    // printable ASCII when Code is zero
}

// KeyCode names the special keys graphics mode handles.
type KeyCode uint8

// Special keys.
const (
	KeyUp KeyCode = iota + 1
	KeyDown
	KeyRight
	KeyLeft
	KeyEsc
)

// ParseKeys decodes raw console input. Arrow keys arrive as escape
// sequences (ESC [ A, or ESC O A in application mode), so ESC is only a key
// on its own at the end of a read; every other escape sequence is consumed
// whole and ignored, so its bytes are never mistaken for letters.
func ParseKeys(b []byte) []Key {
	var out []Key
	arrow := map[byte]KeyCode{'A': KeyUp, 'B': KeyDown, 'C': KeyRight, 'D': KeyLeft}
	for i := 0; i < len(b); i++ {
		c := b[i]
		if c != 0x1b {
			if c >= 0x20 && c <= 0x7e {
				out = append(out, Key{Rune: c})
			}
			continue
		}
		if i+1 >= len(b) {
			out = append(out, Key{Code: KeyEsc})
			break
		}
		switch b[i+1] {
		case '[': // CSI: parameters 0x30-0x3F, intermediates 0x20-0x2F, final 0x40-0x7E
			j := i + 2
			for j < len(b) && b[j] >= 0x20 && b[j] <= 0x3f {
				j++
			}
			if j >= len(b) {
				return out // truncated sequence: drop it
			}
			if k, ok := arrow[b[j]]; ok {
				out = append(out, Key{Code: k})
			}
			i = j
		case 'O': // SS3: one final byte
			if i+2 >= len(b) {
				return out
			}
			if k, ok := arrow[b[i+2]]; ok {
				out = append(out, Key{Code: k})
			}
			i += 2
		default: // alt+key: ignore both bytes
			i++
		}
	}
	return out
}
