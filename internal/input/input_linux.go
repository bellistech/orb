//go:build linux

package input

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	maxDevices = 32 // devices watched; a desk has a handful

	inputMajor = 13 // /dev/input/* (<linux/major.h>)
	evdevMinor = 64 // first event device; below are js (0..31) and mouse (32..63)

	// EVIOCGBIT(0, 8): the event types a device can send.
	eviocgbitTypes = 0x80084520
)

func isEvdev(major, minor uint32) bool { return major == inputMajor && minor >= evdevMinor }

// Open opens every keyboard and pointer event device in dir for reading.
// Best effort: a device that cannot be opened (not root, not in the input
// group) or is of another kind is skipped. Devices plugged in later are
// not seen.
func Open(dir string) []*os.File {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []*os.File
	for _, e := range ents {
		if len(out) == maxDevices {
			break
		}
		if !strings.HasPrefix(e.Name(), "event") {
			continue
		}
		f, err := openEvdev(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if !human(types(f)) {
			f.Close()
			continue
		}
		out = append(out, f)
	}
	return out
}

// openEvdev opens path read-only and non-blocking, only if it is an
// event device. This may run as root, and opening a device can act on
// its own (opening a watchdog arms it), so the node is resolved once
// with O_PATH, which opens nothing, checked, then reopened through
// /proc/self/fd, which opens that same inode: swapping the path in
// between changes nothing.
func openEvdev(path string) (*os.File, error) {
	pfd, err := unix.Open(path, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	defer unix.Close(pfd)
	var st unix.Stat_t
	if err := unix.Fstat(pfd, &st); err != nil {
		return nil, &os.PathError{Op: "stat", Path: path, Err: err}
	}
	if st.Mode&unix.S_IFMT != unix.S_IFCHR || !isEvdev(unix.Major(st.Rdev), unix.Minor(st.Rdev)) {
		return nil, &os.PathError{Op: "open", Path: path, Err: unix.ENODEV}
	}
	fd, err := unix.Open("/proc/self/fd/"+strconv.Itoa(pfd), unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC|unix.O_NOCTTY, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(fd), path), nil
}

// types asks an event device which event types it sends; 0 if it will
// not say. Through SyscallConn, not Fd: Fd would put the descriptor in
// blocking mode, and then Close could not end a pending Read.
func types(f *os.File) byte {
	rc, err := f.SyscallConn()
	if err != nil {
		return 0
	}
	var bits [8]byte
	var errno unix.Errno
	if err := rc.Control(func(fd uintptr) {
		// The kernel fills the bitmap in place; unsafe is the only way
		// to hand it the address.
		_, _, errno = unix.Syscall(unix.SYS_IOCTL, fd, eviocgbitTypes, uintptr(unsafe.Pointer(&bits[0])))
	}); err != nil || errno != 0 {
		return 0
	}
	return bits[0]
}

// Start opens the devices in dir in this process and returns a channel
// that signals activity, and a function that closes them. For an orb
// that is not root: its user either may read the devices (input group)
// or may not, and there is no privilege to separate. With no device it
// returns a nil channel.
func Start(dir string) (<-chan struct{}, func()) {
	files := Open(dir)
	if len(files) == 0 {
		return nil, func() {}
	}
	ch := make(chan struct{}, 1)
	for _, f := range files {
		go Watch(f, ch, Throttle)
	}
	return ch, func() {
		for _, f := range files {
			f.Close()
		}
	}
}
