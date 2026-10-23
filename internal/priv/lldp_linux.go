//go:build linux

package priv

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"orb/internal/lldp"
)

// StartLLDPHelper re-executes this binary as the LLDP listener, in the
// host network namespace (it must see the links) with an empty
// environment. The helper opens its packet socket as root, then drops to
// nobody with no capabilities at all: reading an open socket needs none.
// Neighbors it reports land in the returned table. Call as root, before
// dropping the main process. The helper exits when its stdin closes.
func StartLLDPHelper() (*lldp.Table, func(), error) {
	cmd := exec.Command("/proc/self/exe", LLDPHelperFlag)
	cmd.Env = []string{}
	w, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	r, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start lldp helper: %w", err)
	}
	go cmd.Wait() // reap it whenever it exits
	stop := func() { w.Close() }
	br := bufio.NewReaderSize(r, maxLLDPLine+1)
	ready := make(chan bool, 1)
	go func() {
		line, err := br.ReadString('\n')
		ready <- err == nil && strings.TrimSuffix(line, "\n") == lldpReady
	}()
	select {
	case ok := <-ready:
		if !ok {
			stop()
			return nil, nil, errors.New("lldp helper did not start")
		}
	case <-time.After(5 * time.Second):
		stop()
		return nil, nil, errors.New("lldp helper did not start")
	}
	t := lldp.NewTable(true)
	go func() {
		_ = pumpLLDP(br, t) // ends with the helper; a bad line ends it too
		stop()
	}()
	return t, stop, nil
}

// RunLLDPHelper is the helper's whole life: open the socket and join the
// LLDP group as root, drop to nobody with nothing kept, report neighbors
// until stdin closes.
func RunLLDPHelper(in io.Reader, out io.Writer) error {
	if os.Geteuid() != 0 {
		return errors.New("the lldp helper must start as root")
	}
	l, err := lldp.Open()
	if err != nil {
		return fmt.Errorf("lldp socket: %w", err)
	}
	l.Join()
	if err := Drop(nobody); err != nil {
		return err
	}
	if _, err := io.WriteString(out, lldpReady+"\n"); err != nil {
		return err
	}
	// The parent closing stdin is the signal to stop.
	go func() {
		_, _ = io.Copy(io.Discard, in)
		os.Exit(0)
	}()
	return l.Run(func(iface string, n lldp.Neighbor) {
		if writeLLDP(out, iface, n) != nil {
			os.Exit(0) // parent gone
		}
	})
}
