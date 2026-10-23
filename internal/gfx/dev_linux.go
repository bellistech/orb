//go:build linux

package gfx

import (
	"fmt"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

// Device majors orb may open read-write (<linux/major.h>).
const (
	ttyMajor    = 4  // VTs (tty0..tty63)
	ttyAuxMajor = 5  // /dev/tty (0), /dev/console (1)
	fbMajor     = 29 // /dev/fbN
)

func isFB(major, _ uint32) bool { return major == fbMajor }

func isConsole(major, minor uint32) bool {
	return major == ttyMajor || major == ttyAuxMajor && minor <= 1
}

// openDevice opens path read-write only if it is a character device that
// ok accepts. Run as root with a path from the command line, it must not
// open just anything: opening a device can act on its own (opening a
// watchdog arms it). The node is resolved once with O_PATH, which does
// not open the device, checked, then reopened through /proc/self/fd,
// which opens that same inode: swapping the path in between changes
// nothing.
func openDevice(path, what string, ok func(major, minor uint32) bool) (*os.File, error) {
	pfd, err := unix.Open(path, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	defer unix.Close(pfd)
	var st unix.Stat_t
	if err := unix.Fstat(pfd, &st); err != nil {
		return nil, &os.PathError{Op: "stat", Path: path, Err: err}
	}
	if st.Mode&unix.S_IFMT != unix.S_IFCHR || !ok(unix.Major(st.Rdev), unix.Minor(st.Rdev)) {
		return nil, fmt.Errorf("%s is not %s", path, what)
	}
	fd, err := unix.Open("/proc/self/fd/"+strconv.Itoa(pfd), unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOCTTY, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(fd), path), nil
}
