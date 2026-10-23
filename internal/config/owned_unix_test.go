//go:build unix

package config

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// As root, orb reads the invoking user's config, a path that user
// controls. Only a regular file of theirs may be opened: not a symlink to
// another file, not a FIFO (it would block), not a device.
func TestLoadOwned(t *testing.T) {
	me := os.Getuid()
	good := write(t, "theme = chrono\n", 0o600)
	if c, _, err := LoadOwned(good, me); err != nil || c.Theme != "chrono" {
		t.Fatalf("own regular file: %+v, %v", c, err)
	}
	if c, _, err := LoadOwned(filepath.Join(t.TempDir(), "none"), me); err != nil || c != (Config{}) {
		t.Fatalf("missing file: %+v, %v", c, err)
	}
	if _, _, err := LoadOwned(good, me+1); err == nil {
		t.Error("read a file owned by someone else")
	}

	dir := t.TempDir()
	link := filepath.Join(dir, "config")
	if err := os.Symlink(good, link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadOwned(link, me); err == nil {
		t.Error("followed a symlink")
	}

	fifo := filepath.Join(t.TempDir(), "config")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, _, err := LoadOwned(fifo, me); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Error("read a FIFO")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("blocked opening a FIFO")
	}

	for _, dev := range []string{"/dev/null", "/dev/zero"} {
		if _, _, err := LoadOwned(dev, 0); err == nil {
			t.Errorf("opened %s", dev)
		}
	}
}
