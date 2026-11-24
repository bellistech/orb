package priv

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"orb/internal/sys"
)

// The radio scan helper protocol, helper -> main over the helper's stdout:
// one "R" line once the helper has dropped, then one JSON object per
// passive scan it asked for: {"if": "<interface>", "at": <unix seconds>,
// "err": "<word>"}. err is "" when the radio accepted the scan, else one
// of scanErrs. The helper reads nothing from the air: it only asks the
// kernel to listen. Scan results are read by the unprivileged main
// process (radio.Heard).

// RadioHelperFlag is the hidden argument that runs the radio scan helper.
const RadioHelperFlag = "-radio-helper"

// CapNetAdmin lets the helper ask nl80211 for a scan.
const CapNetAdmin Cap = 12

// Radio scan intervals the helper accepts.
const (
	RadioScanMin = 30 * time.Second
	RadioScanMax = time.Hour
)

// CheckRadioScan reports whether d is a valid scan interval: 0 (off) or
// RadioScanMin..RadioScanMax.
func CheckRadioScan(d time.Duration) bool {
	return d == 0 || d >= RadioScanMin && d <= RadioScanMax
}

const (
	radioReady   = "R"
	maxRadioLine = 256
)

var scanErrs = map[string]bool{"": true, "busy": true, "down": true, "denied": true, "unsupported": true, "failed": true}

type radioMsg struct {
	If  string `json:"if"`
	At  int64  `json:"at"`
	Err string `json:"err"`
}

func writeRadio(w io.Writer, iface string, at time.Time, word string) error {
	b, err := json.Marshal(radioMsg{If: iface, At: at.Unix(), Err: word})
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

// readRadio parses one line strictly.
func readRadio(line []byte, now time.Time) (string, sys.ScanInfo, error) {
	var m radioMsg
	if len(line) > maxRadioLine || json.Unmarshal(line, &m) != nil || !validIf(m.If) || !scanErrs[m.Err] ||
		m.At <= 0 || time.Unix(m.At, 0).After(now.Add(time.Minute)) {
		return "", sys.ScanInfo{}, errors.New("radio helper: malformed line")
	}
	return m.If, sys.ScanInfo{At: time.Unix(m.At, 0), Err: m.Err}, nil
}

// RadioScans is the main process's view of the helper's reports.
type RadioScans struct {
	every time.Duration
	mu    sync.Mutex
	last  map[string]sys.ScanInfo
	alive bool
}

// State is the scan state for the sampler (sys.Sampler.RadioScan).
func (r *RadioScans) State() sys.ScanState {
	r.mu.Lock()
	defer r.mu.Unlock()
	st := sys.ScanState{On: r.alive, Every: r.every, Last: make(map[string]sys.ScanInfo, len(r.last))}
	for k, v := range r.last {
		st.Last[k] = v
	}
	return st
}

// pumpRadio records the helper's reports until it ends or misbehaves.
func pumpRadio(br *bufio.Reader, r *RadioScans) error {
	for {
		line, err := br.ReadSlice('\n')
		if err != nil {
			return err
		}
		iface, info, err := readRadio(line[:len(line)-1], time.Now())
		if err != nil {
			return err
		}
		r.mu.Lock()
		if len(r.last) < 64 || r.last[iface] != (sys.ScanInfo{}) {
			r.last[iface] = info
		}
		r.mu.Unlock()
	}
}
