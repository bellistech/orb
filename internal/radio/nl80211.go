package radio

import (
	"encoding/binary"
	"time"
)

// Generic netlink and nl80211 numbers (<linux/genetlink.h>,
// <linux/nl80211.h>). The ABI is stable: numbers are only ever added.
const (
	GenlIDCtrl          = 0x10
	CtrlCmdGetFamily    = 3
	CtrlAttrFamilyID    = 1
	CtrlAttrFamilyName  = 2
	GenlHdrLen          = 4
	CmdGetInterface     = 5
	CmdNewInterface     = 7
	CmdGetScan          = 32
	CmdTriggerScan      = 33
	CmdNewScanResults   = 34
	AttrWiphy           = 1
	AttrIfindex         = 3
	AttrIfname          = 4
	AttrIftype          = 5
	AttrMAC             = 6
	AttrWiphyFreq       = 38
	AttrBSS             = 47
	bssBSSID            = 1
	bssFrequency        = 2
	bssCapability       = 5
	bssInformationElems = 6
	bssSignalMBM        = 7
	bssStatus           = 9
	bssSeenMsAgo        = 10
	bssBeaconIEs        = 11
	nlaTypeMask         = 0x3fff
	maxAttrs            = 512 // per level: a BSS has about 15
)

// Attrs walks netlink attributes in b, calling f with each type (flags
// masked off) and payload. It stops at the first malformed header and
// after maxAttrs, so hostile input costs at most that much.
func Attrs(b []byte, f func(typ uint16, v []byte)) {
	for n := 0; len(b) >= 4 && n < maxAttrs; n++ {
		l := int(binary.NativeEndian.Uint16(b[0:2]))
		if l < 4 || l > len(b) {
			return
		}
		f(binary.NativeEndian.Uint16(b[2:4])&nlaTypeMask, b[4:l])
		adv := (l + 3) &^ 3
		if adv > len(b) {
			return
		}
		b = b[adv:]
	}
}

func u32(v []byte) (uint32, bool) {
	if len(v) < 4 {
		return 0, false
	}
	return binary.NativeEndian.Uint32(v), true
}

// cstr is a NUL-terminated attribute string, printable and capped.
func cstr(v []byte, max int) string {
	for i, c := range v {
		if c == 0 {
			v = v[:i]
			break
		}
	}
	return printable(v, max)
}

var iftypes = map[uint32]string{1: "adhoc", 2: "station", 3: "ap", 6: "monitor", 7: "mesh", 8: "p2p-client", 9: "p2p-go", 10: "p2p-device"}

// ParseInterface decodes one NL80211_CMD_NEW_INTERFACE payload (after the
// genetlink header).
func ParseInterface(b []byte) (Iface, bool) {
	var i Iface
	var gotIdx bool
	Attrs(b, func(t uint16, v []byte) {
		switch t {
		case AttrIfindex:
			if x, ok := u32(v); ok && x > 0 && x < 1<<24 {
				i.Index, gotIdx = int(x), true
			}
		case AttrIfname:
			i.Name = cstr(v, 15)
		case AttrWiphy:
			if x, ok := u32(v); ok && x < 1<<16 {
				i.Phy = int(x)
			}
		case AttrIftype:
			if x, ok := u32(v); ok {
				if s, known := iftypes[x]; known {
					i.Type = s
				} else {
					i.Type = "other"
				}
			}
		case AttrMAC:
			if len(v) == 6 {
				copy(i.MAC[:], v)
			}
		case AttrWiphyFreq:
			if x, ok := u32(v); ok && x < 100000 {
				i.FreqMHz = int(x)
			}
		}
	})
	return i, gotIdx && i.Name != ""
}

// ParseBSS decodes one NL80211_CMD_NEW_SCAN_RESULTS payload (after the
// genetlink header): the nested NL80211_ATTR_BSS. now is when the dump was
// read; the kernel reports how long ago each BSS was last heard.
func ParseBSS(b []byte, now time.Time) (AP, bool) {
	var ap AP
	var ok bool
	Attrs(b, func(t uint16, v []byte) {
		if t != AttrBSS {
			return
		}
		var ies, beacon []byte
		var capab uint16
		var gotBSSID, gotFreq bool
		Attrs(v, func(t uint16, v []byte) {
			switch t {
			case bssBSSID:
				if len(v) == 6 {
					copy(ap.BSSID[:], v)
					gotBSSID = true
				}
			case bssFrequency:
				if x, k := u32(v); k && x > 0 && x < 100000 {
					ap.FreqMHz, gotFreq = int(x), true
				}
			case bssCapability:
				if len(v) >= 2 {
					capab = binary.NativeEndian.Uint16(v)
				}
			case bssInformationElems:
				ies = v
			case bssBeaconIEs:
				beacon = v
			case bssSignalMBM:
				if x, k := u32(v); k {
					if dbm := float64(int32(x)) / 100; dbm > -150 && dbm < 30 {
						ap.Signal = dbm
					}
				}
			case bssStatus:
				if x, k := u32(v); k && x == 1 { // associated
					ap.Joined = true
				}
			case bssSeenMsAgo:
				if x, k := u32(v); k && x < 7*24*3600*1000 {
					ap.Seen = now.Add(-time.Duration(x) * time.Millisecond)
				}
			}
		})
		if ies == nil {
			ies = beacon
		}
		ap.SSID, ap.Hidden, ap.Security = elements(ies, capab)
		if ap.Seen.IsZero() {
			ap.Seen = now
		}
		ok = gotBSSID && gotFreq
	})
	return ap, ok
}

// Information element IDs and suite selectors (IEEE 802.11-2020 9.4).
const (
	ieSSID   = 0
	ieRSN    = 48
	ieVendor = 221
	capPriv  = 0x0010
)

// elements reads the SSID and the security a network advertises.
func elements(ies []byte, capab uint16) (ssid string, hidden bool, sec string) {
	var rsn []byte
	wpa := false
	hidden = true
	for n := 0; len(ies) >= 2 && n < 256; n++ {
		id, l := ies[0], int(ies[1])
		if 2+l > len(ies) {
			break
		}
		v := ies[2 : 2+l]
		switch id {
		case ieSSID:
			if l <= 32 {
				for _, c := range v {
					if c != 0 {
						hidden = false
						break
					}
				}
				if !hidden {
					ssid = printable(v, 32)
				}
			}
		case ieRSN:
			rsn = v
		case ieVendor:
			if l >= 4 && v[0] == 0x00 && v[1] == 0x50 && v[2] == 0xf2 && v[3] == 1 {
				wpa = true
			}
		}
		ies = ies[2+l:]
	}
	switch {
	case rsn != nil:
		sec = rsnSecurity(rsn)
	case wpa:
		sec = "WPA"
	case capab&capPriv != 0:
		sec = "WEP"
	default:
		sec = "open"
	}
	return ssid, hidden, sec
}

// rsnSecurity names an RSN element by its key management suites.
func rsnSecurity(v []byte) string {
	// version(2) group cipher(4) pairwise count(2) + 4n, AKM count(2) + 4m
	if len(v) < 8 {
		return "WPA2"
	}
	p := 6
	np := int(binary.LittleEndian.Uint16(v[p : p+2]))
	p += 2 + 4*np
	if np > 64 || p+2 > len(v) {
		return "WPA2"
	}
	na := int(binary.LittleEndian.Uint16(v[p : p+2]))
	p += 2
	var psk, sae, eap, eap3, owe bool
	for i := 0; i < na && i < 64 && p+4 <= len(v); i, p = i+1, p+4 {
		if v[p] != 0x00 || v[p+1] != 0x0f || v[p+2] != 0xac {
			continue
		}
		switch v[p+3] {
		case 1, 3, 5:
			eap = true
		case 2, 4, 6:
			psk = true
		case 8, 9, 24, 25:
			sae = true
		case 12, 13:
			eap3 = true
		case 18:
			owe = true
		}
	}
	switch {
	case eap3:
		return "WPA3-EAP"
	case sae && psk:
		return "WPA2/3"
	case sae:
		return "WPA3"
	case owe:
		return "OWE"
	case eap:
		return "WPA2-EAP"
	}
	return "WPA2"
}
