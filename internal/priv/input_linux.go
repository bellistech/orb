//go:build linux

package priv

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"orb/internal/input"
)

// StartInputHelper re-executes this binary as the input helper, in a new
// network namespace with no interfaces, with an empty environment. The
// helper opens the keyboard and pointer event devices as root, then drops
// to nobody with no capabilities: reading an open device needs none. The
// returned channel signals input activity and nothing else, so the main
// process, which parses feeds, never holds a descriptor that could read
// keystrokes. Call as root, before dropping the main process. The helper
// exits when its stdin closes.
func StartInputHelper() (<-chan struct{}, func(), error) {
	cmd := exec.Command("/proc/self/exe", InputHelperFlag)
	cmd.Env = []string{}
	cmd.SysProcAttr = &syscall.SysProcAttr{Cloneflags: syscall.CLONE_NEWNET}
	w, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	r, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start input helper: %w", err)
	}
	go cmd.Wait() // reap it whenever it exits
	stop := func() { w.Close() }
	ready := make(chan bool, 1)
	go func() {
		buf := make([]byte, len(inputReady))
		_, err := io.ReadFull(r, buf)
		ready <- err == nil && string(buf) == inputReady
	}()
	select {
	case ok := <-ready:
		if !ok {
			stop()
			return nil, nil, errors.New("input helper did not start (no keyboard or mouse found?)")
		}
	case <-time.After(5 * time.Second):
		stop()
		return nil, nil, errors.New("input helper did not start")
	}
	ch := make(chan struct{}, 1)
	go func() {
		_ = pumpInput(r, ch) // ends with the helper; a bad byte ends it too
		stop()
	}()
	return ch, stop, nil
}

// RunInputHelper is the helper's whole life: open the event devices as
// root, drop to nobody with nothing kept, write a tick per burst of
// activity until stdin closes. It never looks at what it reads.
func RunInputHelper(in io.Reader, out io.Writer) error {
	if os.Geteuid() != 0 {
		return errors.New("the input helper must start as root")
	}
	files := input.Open(input.Dir)
	if len(files) == 0 {
		return errors.New("no keyboard or pointer under " + input.Dir)
	}
	if err := Drop(nobody); err != nil {
		return err
	}
	if _, err := io.WriteString(out, inputReady); err != nil {
		return err
	}
	// The parent closing stdin is the signal to stop.
	go func() {
		_, _ = io.Copy(io.Discard, in)
		os.Exit(0)
	}()
	ch := make(chan struct{}, 1)
	for _, f := range files {
		go input.Watch(f, ch, input.Throttle)
	}
	for range ch {
		if _, err := out.Write([]byte{inputTick}); err != nil {
			os.Exit(0) // parent gone
		}
	}
	return nil
}
