package sys

import (
	"os"
	"strconv"
	"strings"
	"time"

	"orb/internal/radio"
)

// Radio is the airwaves page: the host's radios and what they have heard.
type Radio struct {
	Ifaces []RadioIface
	APs    []radio.AP // heard in the last RadioKeep, joined first, strongest next
	Scan   ScanState
	Err    string // why nothing could be read, in a few words; "" when fine
}

// RadioIface is one local Wi-Fi radio.
type RadioIface struct {
	radio.Iface
	Up   bool
	Scan ScanInfo // the last passive scan orb asked of it
}

// ScanInfo is the outcome of the last passive scan on one radio.
type ScanInfo struct {
	At  time.Time // zero: none yet
	Err string    // "" when the radio accepted it
}

// ScanState is what the scan helper reports: whether orb scans at all, how
// often, and the last outcome per interface name.
type ScanState struct {
	On    bool
	Every time.Duration
	Last  map[string]ScanInfo
}

// RadioKeep is how long a network stays on the page after it was last
// heard; the kernel forgets it after about 30 s.
const RadioKeep = 10 * time.Minute

// radioEvery is how often the radio state is re-read while wanted.
const radioEvery = 2 * time.Second

func (s *Sampler) readRadio(now time.Time) *Radio {
	if s.radio != nil && now.Sub(s.radioAt) < radioEvery {
		return s.radio
	}
	if s.radioTable == nil {
		s.radioTable = radio.NewTable(RadioKeep)
	}
	v := &Radio{}
	if s.RadioScan != nil {
		v.Scan = s.RadioScan()
	}
	ifs, err := radio.Interfaces()
	if err != nil && len(ifs) == 0 {
		v.Err = brief(err)
	}
	for _, i := range ifs {
		ri := RadioIface{Iface: i, Up: adminUp(i.Name)}
		if v.Scan.Last != nil {
			ri.Scan = v.Scan.Last[i.Name]
		}
		v.Ifaces = append(v.Ifaces, ri)
		if aps, err := radio.Heard(i.Index, i.Name, now); err == nil || len(aps) > 0 {
			s.radioTable.Heard(aps)
		}
	}
	v.APs = s.radioTable.List(now)
	s.radio, s.radioAt = v, now
	return v
}

// adminUp reads IFF_UP from sysfs: switched on, with or without a carrier (a Wi-Fi radio that only listens has none).
func adminUp(name string) bool {
	if name == "" || strings.ContainsAny(name, "/.") {
		return false
	}
	b, err := os.ReadFile("/sys/class/net/" + name + "/flags")
	if err != nil {
		return false
	}
	f, err := strconv.ParseUint(strings.TrimPrefix(strings.TrimSpace(string(b)), "0x"), 16, 32)
	return err == nil && f&1 != 0
}

// brief turns a read error into a few words for the screen.
func brief(err error) string {
	s := err.Error()
	switch {
	case strings.Contains(s, "no nl80211"):
		return "no Wi-Fi driver"
	case strings.Contains(s, "needs Linux"):
		return "needs Linux"
	}
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}
