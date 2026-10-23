//go:build linux

package gfx

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

// opened reports whether anything opened path while fn ran (inotify
// IN_OPEN; resolving a path with O_PATH is not an open).
func opened(t *testing.T, path string, fn func()) bool {
	t.Helper()
	in, err := unix.InotifyInit1(unix.IN_NONBLOCK | unix.IN_CLOEXEC)
	if err != nil {
		t.Skip("inotify:", err)
	}
	defer unix.Close(in)
	if _, err := unix.InotifyAddWatch(in, path, unix.IN_OPEN); err != nil {
		t.Skip("inotify watch:", err)
	}
	fn()
	buf := make([]byte, 4096)
	n, _ := unix.Read(in, buf)
	return n > 0
}

// -fb and -reset-console take a path from the command line and may run as
// root: anything but a framebuffer, or a console, is refused before it is
// opened.
func TestDevicePathsAreNotOpened(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "fb0")
	if err := os.WriteFile(plain, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(dir, "tty1")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(plain, link); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{plain, fifo, link} {
		target := p
		if p == link {
			target = plain
		}
		var errFB, errTTY error
		if opened(t, target, func() {
			_, errFB = OpenFB(p)
			errTTY = ResetConsole(p)
		}) {
			t.Errorf("%s was opened", p)
		}
		if errFB == nil || !strings.Contains(errFB.Error(), "not a framebuffer") {
			t.Errorf("OpenFB(%s): %v", p, errFB)
		}
		if errTTY == nil || !strings.Contains(errTTY.Error(), "not a virtual console") {
			t.Errorf("ResetConsole(%s): %v", p, errTTY)
		}
	}
	// A character device of the wrong kind.
	if _, err := OpenFB("/dev/null"); err == nil || !strings.Contains(err.Error(), "not a framebuffer") {
		t.Errorf("OpenFB(/dev/null): %v", err)
	}
	if err := ResetConsole("/dev/null"); err == nil || !strings.Contains(err.Error(), "not a virtual console") {
		t.Errorf("ResetConsole(/dev/null): %v", err)
	}
}

// The checked node is the one opened, and it is usable.
func TestOpenDevice(t *testing.T) {
	f, err := openDevice("/dev/null", "null", func(major, minor uint32) bool { return major == 1 && minor == 3 })
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if n, err := f.Write([]byte("x")); n != 1 || err != nil {
		t.Fatalf("write: %d, %v", n, err)
	}
	if f.Name() != "/dev/null" {
		t.Errorf("name %q", f.Name())
	}
}
