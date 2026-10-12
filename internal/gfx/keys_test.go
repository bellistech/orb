package gfx

import (
	"reflect"
	"testing"
)

func TestParseKeys(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []Key
	}{
		{"q", []Key{{Rune: 'q'}}},
		{"\x1b[A", []Key{{Code: KeyUp}}},
		{"\x1b[B\x1b[C\x1b[D", []Key{{Code: KeyDown}, {Code: KeyRight}, {Code: KeyLeft}}},
		{"\x1bOA", []Key{{Code: KeyUp}}},    // application cursor mode
		{"\x1b[1;5A", []Key{{Code: KeyUp}}}, // with modifiers
		{"\x1b[15~t", []Key{{Rune: 't'}}},   // F5 swallowed, t kept
		{"\x1b", []Key{{Code: KeyEsc}}},     // lone escape
		{"\x1bx", nil},                      // alt+x ignored
		{"\x1b[", nil},                      // truncated: dropped, never a quit
		{" w", []Key{{Rune: ' '}, {Rune: 'w'}}},
		{"\x00\x7f", nil}, // controls dropped
	} {
		if got := ParseKeys([]byte(tc.in)); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ParseKeys(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

func FuzzParseKeys(f *testing.F) {
	f.Add([]byte("\x1b[1;5A q"))
	f.Fuzz(func(t *testing.T, b []byte) {
		for _, k := range ParseKeys(b) {
			if k.Code == 0 && (k.Rune < 0x20 || k.Rune > 0x7e) {
				t.Fatalf("bad key %+v", k)
			}
		}
	})
}
