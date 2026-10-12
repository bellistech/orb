//go:build linux

package gfx

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ioctl numbers from <linux/kd.h> and <linux/vt.h>.
const (
	kdSetMode    = 0x4B3A
	kdGetMode    = 0x4B3B
	kdText       = 0
	kdGraphics   = 1
	vtOpenQry    = 0x5600
	vtGetMode    = 0x5601
	vtSetMode    = 0x5602
	vtGetState   = 0x5603
	vtRelDisp    = 0x5605
	vtActivate   = 0x5606
	vtWaitActive = 0x5607
	vtProcess    = 1
	vtAckAcq     = 2
	sigRelease   = syscall.SIGUSR1
	sigAcquire   = syscall.SIGUSR2
)

// vtMode mirrors struct vt_mode.
type vtMode struct {
	Mode, Waitv           int8
	Relsig, Acqsig, Frsig int16
}

// ErrNotConsole means orb is not on a Linux VT (SSH, a pseudo-terminal
// such as sudo-rs or tmux gives) and is not root, so it cannot take one.
var ErrNotConsole = errors.New("not on a Linux virtual console: run -gfx from the console login, or as root (sudo orb -gfx) to take a free console; over SSH drop -gfx")

// vtStat mirrors struct vt_stat.
type vtStat struct{ Active, Signal, State uint16 }

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

	// Set when orb took a free VT rather than running on its own: the
	// VT to switch back to on exit, through tty0.
	tty0     *os.File
	returnTo int
}

// OpenConsole takes over the controlling VT. The caller must already be
// notified of sigRelease and sigAcquire, or a VT switch would kill the
// process with the console left in graphics mode.
func OpenConsole() (*Console, error) {
	c, err := ownConsole()
	if err != nil {
		return nil, err
	}
	fd := int(c.f.Fd())
	var errSetup error
	if c.termios, errSetup = unix.IoctlGetTermios(fd, unix.TCGETS); errSetup != nil {
		c.closeAll()
		return nil, fmt.Errorf("read terminal mode: %w", errSetup)
	}
	if err := ioctlPtr(fd, vtGetMode, unsafe.Pointer(&c.vt)); err != nil {
		c.closeAll()
		return nil, fmt.Errorf("VT_GETMODE: %w", err)
	}

	// Raw keys, signals off: Ctrl+C arrives as a byte and is handled as
	// quit, and Ctrl+Z cannot suspend orb with the screen in graphics.
	raw := *c.termios
	raw.Lflag &^= unix.ICANON | unix.ECHO | unix.ISIG
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

// ownConsole returns the VT to draw on: the controlling terminal when it is
// one, otherwise (root only) a free VT that it switches the display to.
func ownConsole() (*Console, error) {
	if f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		if kd, err := unix.IoctlGetInt(int(f.Fd()), kdGetMode); err == nil {
			return &Console{f: f, kd: kd}, nil
		}
		f.Close()
	}
	tty0, err := os.OpenFile("/dev/tty0", os.O_RDWR, 0)
	if err != nil {
		return nil, ErrNotConsole
	}
	fail := func(format string, a ...any) (*Console, error) {
		tty0.Close()
		return nil, fmt.Errorf(format, a...)
	}
	var st vtStat
	if err := ioctlPtr(int(tty0.Fd()), vtGetState, unsafe.Pointer(&st)); err != nil {
		return fail("VT_GETSTATE: %w", err)
	}
	n, err := unix.IoctlGetInt(int(tty0.Fd()), vtOpenQry)
	if err != nil || n <= 0 || n > 63 {
		return fail("no free virtual console (VT_OPENQRY: %v)", err)
	}
	f, err := os.OpenFile(fmt.Sprintf("/dev/tty%d", n), os.O_RDWR, 0)
	if err != nil {
		return fail("open /dev/tty%d: %w", n, err)
	}
	kd, err := unix.IoctlGetInt(int(f.Fd()), kdGetMode)
	if err != nil {
		f.Close()
		return fail("KDGETMODE on tty%d: %w", n, err)
	}
	c := &Console{f: f, kd: kd, tty0: tty0, returnTo: int(st.Active)}
	if err := switchVT(tty0, n); err != nil {
		c.closeAll()
		return nil, err
	}
	return c, nil
}

// switchVT shows VT n and waits (bounded) for the switch to happen.
func switchVT(tty0 *os.File, n int) error {
	fd := int(tty0.Fd())
	if err := unix.IoctlSetInt(fd, vtActivate, n); err != nil {
		return fmt.Errorf("VT_ACTIVATE %d: %w", n, err)
	}
	done := make(chan error, 1)
	go func() { done <- unix.IoctlSetInt(fd, vtWaitActive, n) }()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("VT_WAITACTIVE %d: %w", n, err)
		}
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("switching to tty%d timed out (another VT refused)", n)
	}
}

func (c *Console) closeAll() {
	c.f.Close()
	if c.tty0 != nil {
		c.tty0.Close()
	}
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
		if c.tty0 != nil {
			// Hand the display back to the console we took it from.
			if c.returnTo > 0 {
				errs = append(errs, unix.IoctlSetInt(int(c.tty0.Fd()), vtActivate, c.returnTo))
			}
			errs = append(errs, c.tty0.Close())
		}
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
