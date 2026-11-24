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
	"syscall"
	"time"

	"orb/internal/radio"
	"orb/internal/sys"
)

// StartRadioHelper re-executes this binary as the passive scan helper, in
// the host network namespace (it must see the radios) with an empty
// environment. The helper drops to nobody keeping only CAP_NET_ADMIN,
// which nl80211 checks on every scan request, and asks each Wi-Fi
// interface that is up for a passive scan every interval. Call as root,
// before dropping the main process. The helper exits when its stdin
// closes.
func StartRadioHelper(every time.Duration) (*RadioScans, func(), error) {
	if every <= 0 || !CheckRadioScan(every) {
		return nil, nil, errors.New("radio scan interval out of range")
	}
	cmd := exec.Command("/proc/self/exe", RadioHelperFlag, every.String())
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
		return nil, nil, fmt.Errorf("start radio helper: %w", err)
	}
	go cmd.Wait()
	scans := &RadioScans{every: every, last: map[string]sys.ScanInfo{}}
	stop := func() { w.Close() }
	br := bufio.NewReaderSize(r, maxRadioLine+1)
	ready := make(chan bool, 1)
	go func() {
		line, err := br.ReadString('\n')
		ready <- err == nil && strings.TrimSuffix(line, "\n") == radioReady
	}()
	select {
	case ok := <-ready:
		if !ok {
			stop()
			return nil, nil, errors.New("radio helper did not start")
		}
	case <-time.After(5 * time.Second):
		stop()
		return nil, nil, errors.New("radio helper did not start")
	}
	scans.alive = true
	go func() {
		_ = pumpRadio(br, scans)
		scans.mu.Lock()
		scans.alive = false
		scans.mu.Unlock()
		stop()
	}()
	return scans, stop, nil
}

// RunRadioHelper is the helper's whole life: drop to nobody keeping only
// CAP_NET_ADMIN, then ask for a passive scan on every station interface
// every interval until stdin closes. args is the interval.
func RunRadioHelper(args []string, in io.Reader, out io.Writer) error {
	if os.Geteuid() != 0 {
		return errors.New("the radio helper must start as root")
	}
	if len(args) != 1 {
		return errors.New("radio helper: want one interval")
	}
	every, err := time.ParseDuration(args[0])
	if err != nil || every <= 0 || !CheckRadioScan(every) {
		return errors.New("radio helper: bad interval")
	}
	if err := Drop(nobody, CapNetAdmin); err != nil {
		return err
	}
	if _, err := io.WriteString(out, radioReady+"\n"); err != nil {
		return err
	}
	go func() {
		_, _ = io.Copy(io.Discard, in)
		os.Exit(0)
	}()
	for {
		ifs, _ := radio.Interfaces()
		for _, i := range ifs {
			if i.Type != "station" {
				continue // an access point or monitor interface is not ours to retune
			}
			if writeRadio(out, i.Name, time.Now(), scanWord(radio.TriggerPassive(i.Index))) != nil {
				os.Exit(0) // parent gone
			}
		}
		time.Sleep(every)
	}
}

// scanWord reduces a trigger error to one word of the protocol.
func scanWord(err error) string {
	var e syscall.Errno
	switch {
	case err == nil:
		return ""
	case errors.As(err, &e) && e == syscall.EBUSY:
		return "busy"
	case errors.As(err, &e) && (e == syscall.ENETDOWN || e == syscall.ENOLINK):
		return "down"
	case errors.As(err, &e) && (e == syscall.EPERM || e == syscall.EACCES):
		return "denied"
	case errors.As(err, &e) && (e == syscall.EOPNOTSUPP || e == syscall.EINVAL):
		return "unsupported"
	}
	return "failed"
}
