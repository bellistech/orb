//go:build linux

package input

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsEvdev(t *testing.T) {
	for _, c := range []struct {
		major, minor uint32
		want         bool
	}{
		{13, 64, true}, {13, 95, true}, {13, 300, true},
		{13, 63, false}, // /dev/input/mice
		{13, 0, false},  // js0
		{4, 64, false}, {1, 3, false},
	} {
		if got := isEvdev(c.major, c.minor); got != c.want {
			t.Errorf("isEvdev(%d, %d) = %v", c.major, c.minor, got)
		}
	}
}

// Only real event devices are opened: not a regular file with the right
// name, not another device behind a symlink, not other names.
func TestOpenRejects(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "event0"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/dev/null", filepath.Join(dir, "event1")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/dev/zero", filepath.Join(dir, "mouse0")); err != nil {
		t.Fatal(err)
	}
	if got := Open(dir); len(got) != 0 {
		t.Fatalf("opened %d files", len(got))
	}
	if got := Open(filepath.Join(dir, "missing")); got != nil {
		t.Fatalf("missing directory: %v", got)
	}
	if ch, stop := Start(dir); ch != nil {
		t.Fatal("Start returned a channel with no device")
	} else {
		stop()
	}
}
