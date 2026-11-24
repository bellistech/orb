// Package radio decodes what the kernel's Wi-Fi stack (nl80211) has heard
// on the air: nearby networks from their beacons, and the host's own
// radios. It only listens: nothing here joins a network or sends a frame.
// Every byte of a beacon comes from whoever is in range, so every decoder
// checks every length and keeps only printable ASCII.
package radio

import (
	"fmt"
	"time"
)

// AP is one network heard: a BSS (a radio of an access point).
type AP struct {
	BSSID    [6]byte
	SSID     string // printable ASCII, "" when hidden
	Hidden   bool
	FreqMHz  int
	Signal   float64 // dBm; 0 when the driver gives none
	Security string  // open, WEP, WPA, WPA2, WPA3, WPA2/3, OWE, WPA2-EAP, WPA3-EAP
	Seen     time.Time
	Iface    string // the local radio that heard it
	Joined   bool   // the local radio is associated with it
}

// Iface is one local Wi-Fi radio interface.
type Iface struct {
	Name    string
	Index   int
	Phy     int
	Type    string // station, ap, monitor, mesh, p2p, other
	MAC     [6]byte
	FreqMHz int // the channel in use when associated, else 0
}

// MAC formats a hardware address the usual way.
func MAC(a [6]byte) string {
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", a[0], a[1], a[2], a[3], a[4], a[5])
}

// Band names the band of a centre frequency.
func Band(mhz int) string {
	switch {
	case mhz >= 2400 && mhz < 2500:
		return "2.4"
	case mhz >= 5150 && mhz < 5925:
		return "5"
	case mhz >= 5925 && mhz < 7125:
		return "6"
	case mhz >= 57000 && mhz < 71000:
		return "60"
	}
	return "?"
}

// Channel is the IEEE channel number of a centre frequency, 0 if unknown.
func Channel(mhz int) int {
	switch {
	case mhz == 2484:
		return 14
	case mhz >= 2412 && mhz <= 2472 && (mhz-2407)%5 == 0:
		return (mhz - 2407) / 5
	case mhz == 5935:
		return 2
	case mhz >= 5955 && mhz < 7125 && (mhz-5950)%5 == 0:
		return (mhz - 5950) / 5
	case mhz >= 5000 && mhz < 5925 && mhz%5 == 0:
		return (mhz - 5000) / 5
	}
	return 0
}

// printable keeps printable ASCII and turns everything else into '?',
// capped at max bytes. SSIDs are 0-32 arbitrary bytes from the air.
func printable(b []byte, max int) string {
	if len(b) > max {
		b = b[:max]
	}
	out := make([]byte, len(b))
	for i, c := range b {
		if c >= 0x20 && c <= 0x7e {
			out[i] = c
		} else {
			out[i] = '?'
		}
	}
	return string(out)
}
