//go:build linux

package gfx

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ioctl numbers from <linux/kd.h> and <linux/vt.h>.
const (
	kdSetMode  = 0x4B3A
	kdGetMode  = 0x4B3B
	kdText     = 0
	kdGraphics = 1
	vtGetMode  = 0x5601
	vtSetMode  = 0x5602
	vtRelDisp  = 0x5605
	vtProcess  = 1
	vtAckAcq   = 2
	sigRelease = syscall.SIGUSR1
	sigAcquire = syscall.SIGUSR2
)

// vtMode mirrors struct vt_mode.
type vtMode struct {
	Mode, Waitv           int8
	Relsig, Acqsig, Frsig int16
}

// ErrNotConsole means the controlling terminal is not a Linux VT, e.g. an
// SSH session.
var ErrNotConsole = errors.New("not on a Linux virtual console; start -gfx from the monitor's own login, or drop -gfx over SSH")

// Console owns the VT while graphics mode runs: text rendering off, raw
// keyboard, and VT switches handed to us as signals so we stop drawing
// while another VT is showing.
type Console struct {
	f       *os.File
	termios *unix.Termios
	vt      vtMode
	kd      int
	once    sync.Once
	err     error
}

// OpenConsole takes over the controlling VT. The caller must already be
// notified of sigRelease and sigAcquire, or a VT switch would kill the
// process with the console left in graphics mode.
func OpenConsole() (*Console, error) {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, ErrNotConsole
	}
	fd := int(f.Fd())
	kd, err := unix.IoctlGetInt(fd, kdGetMode)
	if err != nil {
		f.Close()
		return nil, ErrNotConsole
	}
	c := &Console{f: f, kd: kd}
	if c.termios, err = unix.IoctlGetTermios(fd, unix.TCGETS); err != nil {
		f.Close()
		return nil, fmt.Errorf("read terminal mode: %w", err)
	}
	if err := ioctlPtr(fd, vtGetMode, unsafe.Pointer(&c.vt)); err != nil {
		f.Close()
		return nil, fmt.Errorf("VT_GETMODE: %w", err)
	}

	raw := *c.termios
	raw.Lflag &^= unix.ICANON | unix.ECHO // keep ISIG: Ctrl+C still quits
	raw.Cc[unix.VMIN], raw.Cc[unix.VTIME] = 1, 0
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &raw); err != nil {
		c.Restore()
		return nil, fmt.Errorf("set raw keyboard: %w", err)
	}
	mode := vtMode{Mode: vtProcess, Relsig: int16(sigRelease), Acqsig: int16(sigAcquire)}
	if err := ioctlPtr(fd, vtSetMode, unsafe.Pointer(&mode)); err != nil {
		c.Restore()
		return nil, fmt.Errorf("VT_SETMODE: %w", err)
	}
	if err := unix.IoctlSetInt(fd, kdSetMode, kdGraphics); err != nil {
		c.Restore()
		return nil, fmt.Errorf("KDSETMODE graphics: %w", err)
	}
	return c, nil
}

func ioctlPtr(fd int, req uint, p unsafe.Pointer) error {
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(p)); e != 0 {
		return e
	}
	return nil
}

// Release lets the kernel switch away to another VT.
func (c *Console) Release() error { return unix.IoctlSetInt(int(c.f.Fd()), vtRelDisp, 1) }

// Acquire acknowledges switching back to our VT.
func (c *Console) Acquire() error { return unix.IoctlSetInt(int(c.f.Fd()), vtRelDisp, vtAckAcq) }

// Read reads keyboard bytes.
func (c *Console) Read(p []byte) (int, error) { return c.f.Read(p) }

// Restore puts the VT back exactly as found. Safe to call more than once.
func (c *Console) Restore() error {
	c.once.Do(func() {
		fd := int(c.f.Fd())
		errs := []error{
			unix.IoctlSetInt(fd, kdSetMode, c.kd),
			ioctlPtr(fd, vtSetMode, unsafe.Pointer(&c.vt)),
			unix.IoctlSetTermios(fd, unix.TCSETS, c.termios),
		}
		// Clear whatever the text console shows when it comes back.
		_, werr := c.f.WriteString("\x1b[H\x1b[2J")
		errs = append(errs, werr, c.f.Close())
		c.err = errors.Join(errs...)
	})
	return c.err
}

// ResetConsole forces a VT back to text mode, for recovering a console left
// in graphics mode by a killed process. tty is e.g. /dev/tty1 ("" means the
// controlling terminal).
func ResetConsole(tty string) error {
	if tty == "" {
		tty = "/dev/tty"
	}
	f, err := os.OpenFile(tty, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", tty, err)
	}
	defer f.Close()
	fd := int(f.Fd())
	auto := vtMode{}
	return errors.Join(
		unix.IoctlSetInt(fd, kdSetMode, kdText),
		ioctlPtr(fd, vtSetMode, unsafe.Pointer(&auto)),
	)
}
