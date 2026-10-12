//go:build linux

package priv

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"orb/internal/sys"
)

// all runs a syscall on every OS thread. Credentials and capabilities are
// per thread on Linux; changing them on one thread would leave the Go
// runtime's other threads privileged.
func all(trap, a1, a2, a3 uintptr) error {
	if _, _, e := syscall.AllThreadsSyscall(trap, a1, a2, a3); e != 0 {
		if e == syscall.ENOTSUP {
			return errors.New("dropping privileges needs a static build (CGO_ENABLED=0)")
		}
		return e
	}
	return nil
}

// Drop becomes t keeping exactly keep, then verifies every thread. Any
// error means the process must not continue: it may be partly privileged.
func Drop(t Target, keep ...Cap) error {
	if t.UID <= 0 || t.GID <= 0 {
		return errors.New("refusing to drop to root")
	}
	mask := Mask(keep...)
	last, err := lastCap()
	if err != nil {
		return err
	}
	// Keep permitted capabilities across the uid change.
	if err := all(unix.SYS_PRCTL, unix.PR_SET_KEEPCAPS, 1, 0); err != nil {
		return fmt.Errorf("PR_SET_KEEPCAPS: %w", err)
	}
	// Shrink the bounding set while CAP_SETPCAP is still effective.
	for c := Cap(0); c <= last; c++ {
		if mask&(1<<c) != 0 {
			continue
		}
		if err := all(unix.SYS_PRCTL, unix.PR_CAPBSET_DROP, uintptr(c), 0); err != nil {
			return fmt.Errorf("PR_CAPBSET_DROP %d: %w", c, err)
		}
	}
	// syscall's Set* functions already apply to all threads.
	if err := syscall.Setgroups(nil); err != nil {
		return fmt.Errorf("setgroups: %w", err)
	}
	if err := syscall.Setresgid(t.GID, t.GID, t.GID); err != nil {
		return fmt.Errorf("setresgid: %w", err)
	}
	if err := syscall.Setresuid(t.UID, t.UID, t.UID); err != nil {
		return fmt.Errorf("setresuid: %w", err)
	}
	hdr := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var data [2]unix.CapUserData
	data[0].Effective, data[0].Permitted = uint32(mask), uint32(mask)
	data[1].Effective, data[1].Permitted = uint32(mask>>32), uint32(mask>>32)
	if err := all(unix.SYS_CAPSET, uintptr(unsafe.Pointer(&hdr)), uintptr(unsafe.Pointer(&data[0])), 0); err != nil {
		return fmt.Errorf("capset: %w", err)
	}
	if err := all(unix.SYS_PRCTL, unix.PR_CAP_AMBIENT, unix.PR_CAP_AMBIENT_CLEAR_ALL, 0); err != nil {
		return fmt.Errorf("clear ambient: %w", err)
	}
	if err := all(unix.SYS_PRCTL, unix.PR_SET_KEEPCAPS, 0, 0); err != nil {
		return fmt.Errorf("PR_SET_KEEPCAPS 0: %w", err)
	}
	if err := all(unix.SYS_PRCTL, unix.PR_SET_NO_NEW_PRIVS, 1, 0); err != nil {
		return fmt.Errorf("PR_SET_NO_NEW_PRIVS: %w", err)
	}
	return verify(t, mask)
}

// verify checks every thread of this process.
func verify(t Target, mask uint64) error {
	tasks, err := filepath.Glob("/proc/self/task/*/status")
	if err != nil || len(tasks) == 0 {
		return fmt.Errorf("cannot list threads to verify the drop: %v", err)
	}
	for _, p := range tasks {
		b, err := os.ReadFile(p)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue // thread exited
			}
			return err
		}
		if err := checkStatus(string(b), t, mask); err != nil {
			return fmt.Errorf("thread %s: %w", filepath.Base(filepath.Dir(p)), err)
		}
	}
	return nil
}

func lastCap() (Cap, error) {
	b, err := os.ReadFile("/proc/sys/kernel/cap_last_cap")
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 8)
	if err != nil || n > 63 {
		return 0, fmt.Errorf("cap_last_cap %q", b)
	}
	return Cap(n), nil
}

// HelperFlag is the hidden argument that runs this binary as the
// socket-owner helper. main checks for it before anything else.
const HelperFlag = "-sock-helper"

// CapDacReadSearch lets the helper list other users' /proc/<pid>/fd.
const CapDacReadSearch Cap = 2

// StartHelper re-executes this binary as the socket-owner helper, in a new
// network namespace with no interfaces, with an empty environment. The
// helper drops to nobody keeping only CAP_DAC_READ_SEARCH and
// CAP_SYS_PTRACE. Call as root, before dropping the main process. The
// helper exits when its stdin closes, i.e. when orb exits.
func StartHelper() (*OwnerClient, error) {
	cmd := exec.Command("/proc/self/exe", HelperFlag)
	cmd.Env = []string{}
	cmd.SysProcAttr = &syscall.SysProcAttr{Cloneflags: syscall.CLONE_NEWNET}
	w, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	r, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start socket helper: %w", err)
	}
	go cmd.Wait() // reap it whenever it exits
	c := &OwnerClient{w: w, r: bufio.NewReaderSize(r, 64<<10), stop: func() { w.Close() }}
	// Prove it dropped and serves before relying on it.
	ok := make(chan bool, 1)
	go func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if _, err := io.WriteString(c.w, requestTag); err != nil {
			ok <- false
			return
		}
		_, err := readOwners(c.r)
		ok <- err == nil
	}()
	select {
	case good := <-ok:
		if good {
			return c, nil
		}
	case <-time.After(5 * time.Second):
	}
	w.Close()
	return nil, errors.New("socket helper did not start")
}

// RunHelper is the helper's whole life: drop, then answer owner requests
// until stdin closes.
func RunHelper(in io.Reader, out io.Writer) error {
	if os.Geteuid() != 0 {
		return errors.New("the socket helper must start as root")
	}
	if err := Drop(nobody, CapDacReadSearch, CapSysPtrace); err != nil {
		return err
	}
	return serve(in, out, sys.LocalOwners)
}
