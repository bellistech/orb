package radio

import (
	"encoding/binary"
	"testing"
	"time"
)

// attr builds one netlink attribute, padded.
func attr(t uint16, v []byte) []byte {
	b := make([]byte, 4, 4+len(v)+3)
	binary.NativeEndian.PutUint16(b[0:2], uint16(4+len(v)))
	binary.NativeEndian.PutUint16(b[2:4], t)
	b = append(b, v...)
	for len(b)%4 != 0 {
		b = append(b, 0)
	}
	return b
}

func u32b(x uint32) []byte { b := make([]byte, 4); binary.NativeEndian.PutUint32(b, x); return b }

func cat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func ie(id byte, v []byte) []byte { return append([]byte{id, byte(len(v))}, v...) }

// rsn builds an RSN element with CCMP and the given AKM suite types.
func rsn(akms ...byte) []byte {
	v := []byte{1, 0, 0x00, 0x0f, 0xac, 4, 1, 0, 0x00, 0x0f, 0xac, 4, byte(len(akms)), 0}
	for _, a := range akms {
		v = append(v, 0x00, 0x0f, 0xac, a)
	}
	return v
}

func bss(bssid []byte, freq uint32, mbm int32, capab uint16, ies []byte, extra ...[]byte) []byte {
	c := make([]byte, 2)
	binary.NativeEndian.PutUint16(c, capab)
	inner := cat(attr(bssBSSID, bssid), attr(bssFrequency, u32b(freq)), attr(bssSignalMBM, u32b(uint32(mbm))),
		attr(bssCapability, c), attr(bssInformationElems, ies), attr(bssSeenMsAgo, u32b(1500)))
	for _, e := range extra {
		inner = append(inner, e...)
	}
	return attr(AttrBSS|0x8000, inner) // NLA_F_NESTED is masked off
}

func TestParseBSS(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	id := []byte{0x02, 0x11, 0x22, 0x33, 0x44, 0x55}
	cases := []struct {
		name, ssid, sec string
		hidden          bool
		ies             []byte
		capab           uint16
	}{
		{"wpa2/3", "Lizard Wizard", "WPA2/3", false, cat(ie(ieSSID, []byte("Lizard Wizard")), ie(ieRSN, rsn(2, 8))), capPriv},
		{"wpa3", "Nonagon", "WPA3", false, cat(ie(ieSSID, []byte("Nonagon")), ie(ieRSN, rsn(8))), capPriv},
		{"wpa2", "home", "WPA2", false, cat(ie(ieSSID, []byte("home")), ie(ieRSN, rsn(2))), capPriv},
		{"enterprise", "corp", "WPA2-EAP", false, cat(ie(ieSSID, []byte("corp")), ie(ieRSN, rsn(1))), capPriv},
		{"owe", "cafe", "OWE", false, cat(ie(ieSSID, []byte("cafe")), ie(ieRSN, rsn(18))), capPriv},
		{"wpa1", "old", "WPA", false, cat(ie(ieSSID, []byte("old")), ie(ieVendor, []byte{0, 0x50, 0xf2, 1, 1, 0})), capPriv},
		{"wep", "older", "WEP", false, ie(ieSSID, []byte("older")), capPriv},
		{"open", "free", "open", false, ie(ieSSID, []byte("free")), 0},
		{"hidden empty", "", "WPA2", true, cat(ie(ieSSID, nil), ie(ieRSN, rsn(2))), capPriv},
		{"hidden zeros", "", "open", true, ie(ieSSID, []byte{0, 0, 0, 0}), 0},
		{"control bytes", "a?b?", "open", false, ie(ieSSID, []byte{'a', 0x1b, 'b', 0xff}), 0},
		{"truncated rsn", "x", "WPA2", false, cat(ie(ieSSID, []byte("x")), ie(ieRSN, []byte{1, 0, 0})), capPriv},
	}
	for _, c := range cases {
		ap, ok := ParseBSS(bss(id, 2437, -6150, c.capab, c.ies), now)
		if !ok {
			t.Fatalf("%s: not parsed", c.name)
		}
		if ap.SSID != c.ssid || ap.Hidden != c.hidden || ap.Security != c.sec {
			t.Errorf("%s: got %q hidden=%v %s", c.name, ap.SSID, ap.Hidden, ap.Security)
		}
		if ap.Signal != -61.5 || ap.FreqMHz != 2437 || MAC(ap.BSSID) != "02:11:22:33:44:55" {
			t.Errorf("%s: got %+v", c.name, ap)
		}
		if !ap.Seen.Equal(now.Add(-1500 * time.Millisecond)) {
			t.Errorf("%s: seen %v", c.name, ap.Seen)
		}
	}
	// Associated status, beacon IEs as a fallback.
	ap, ok := ParseBSS(attr(AttrBSS, cat(attr(bssBSSID, id), attr(bssFrequency, u32b(5180)),
		attr(bssBeaconIEs, ie(ieSSID, []byte("beacon"))), attr(bssStatus, u32b(1)))), now)
	if !ok || !ap.Joined || ap.SSID != "beacon" || !ap.Seen.Equal(now) {
		t.Errorf("joined: %+v %v", ap, ok)
	}
	// Missing BSSID or frequency: refused.
	if _, ok := ParseBSS(attr(AttrBSS, attr(bssFrequency, u32b(2412))), now); ok {
		t.Error("no BSSID accepted")
	}
	if _, ok := ParseBSS(attr(AttrBSS, attr(bssBSSID, id)), now); ok {
		t.Error("no frequency accepted")
	}
	if _, ok := ParseBSS(attr(AttrBSS, cat(attr(bssBSSID, id[:5]), attr(bssFrequency, u32b(2412)))), now); ok {
		t.Error("short BSSID accepted")
	}
}

func TestParseInterface(t *testing.T) {
	b := cat(attr(AttrIfindex, u32b(5)), attr(AttrIfname, []byte("wlp7s0\x00")), attr(AttrWiphy, u32b(0)),
		attr(AttrIftype, u32b(2)), attr(AttrMAC, []byte{0x02, 0x42, 0x0a, 0x1b, 0x00, 0x75}), attr(AttrWiphyFreq, u32b(2412)))
	i, ok := ParseInterface(b)
	if !ok || i.Name != "wlp7s0" || i.Index != 5 || i.Type != "station" || i.FreqMHz != 2412 || MAC(i.MAC) != "02:42:0a:1b:00:75" {
		t.Fatalf("got %+v %v", i, ok)
	}
	if _, ok := ParseInterface(attr(AttrIfname, []byte("x\x00"))); ok {
		t.Error("no index accepted")
	}
	i, _ = ParseInterface(cat(attr(AttrIfindex, u32b(2)), attr(AttrIfname, []byte("w\x00")), attr(AttrIftype, u32b(99))))
	if i.Type != "other" {
		t.Errorf("unknown type = %q", i.Type)
	}
}

func TestAttrsStopsOnBadLength(t *testing.T) {
	n := 0
	Attrs([]byte{2, 0, 1, 0}, func(uint16, []byte) { n++ })         // length below the header
	Attrs([]byte{200, 0, 1, 0, 1, 2}, func(uint16, []byte) { n++ }) // length past the end
	if n != 0 {
		t.Fatalf("walked %d bad attributes", n)
	}
}

func TestChannelBand(t *testing.T) {
	for _, c := range []struct {
		mhz, ch int
		band    string
	}{{2412, 1, "2.4"}, {2437, 6, "2.4"}, {2484, 14, "2.4"}, {5180, 36, "5"}, {5825, 165, "5"}, {5955, 1, "6"}, {6415, 93, "6"}, {5935, 2, "6"}, {900, 0, "?"}} {
		if Channel(c.mhz) != c.ch || Band(c.mhz) != c.band {
			t.Errorf("%d MHz = ch %d band %s, want %d %s", c.mhz, Channel(c.mhz), Band(c.mhz), c.ch, c.band)
		}
	}
}

func TestTableKeepsAndExpires(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	tb := NewTable(10 * time.Minute)
	a := AP{BSSID: [6]byte{2, 0, 0, 0, 0, 1}, SSID: "a", Signal: -70, Seen: now}
	b := AP{BSSID: [6]byte{2, 0, 0, 0, 0, 2}, SSID: "b", Signal: -40, Seen: now}
	j := AP{BSSID: [6]byte{2, 0, 0, 0, 0, 3}, SSID: "j", Signal: -80, Seen: now, Joined: true}
	tb.Heard([]AP{a, b, j})
	l := tb.List(now)
	if len(l) != 3 || l[0].SSID != "j" || l[1].SSID != "b" || l[2].SSID != "a" {
		t.Fatalf("order %+v", l)
	}
	// An older sighting does not overwrite a newer one.
	old := a
	old.Signal, old.Seen = -20, now.Add(-time.Minute)
	tb.Heard([]AP{old})
	if l := tb.List(now); l[2].Signal != -70 {
		t.Fatalf("older sighting won: %+v", l)
	}
	// Past Keep it is gone.
	if l := tb.List(now.Add(11 * time.Minute)); len(l) != 0 {
		t.Fatalf("not expired: %+v", l)
	}
	// Bounded.
	many := make([]AP, maxAPs+50)
	for i := range many {
		many[i] = AP{BSSID: [6]byte{2, 1, 0, 0, byte(i >> 8), byte(i)}, Seen: now}
	}
	tb.Heard(many)
	if l := tb.List(now); len(l) != maxAPs {
		t.Fatalf("table holds %d, cap %d", len(l), maxAPs)
	}
}

func FuzzParseBSS(f *testing.F) {
	id := []byte{2, 1, 2, 3, 4, 5}
	f.Add(bss(id, 2437, -6000, capPriv, cat(ie(ieSSID, []byte("x")), ie(ieRSN, rsn(2, 8)))))
	f.Add(bss(id, 5180, -9000, 0, ie(ieVendor, []byte{0, 0x50, 0xf2, 1})))
	f.Add([]byte{8, 0, 47, 0, 4, 0, 1, 0})
	now := time.Unix(1_800_000_000, 0)
	f.Fuzz(func(t *testing.T, b []byte) {
		ap, ok := ParseBSS(b, now)
		for _, c := range []byte(ap.SSID) {
			if c < 0x20 || c > 0x7e {
				t.Fatalf("unprintable SSID byte %#x", c)
			}
		}
		if len(ap.SSID) > 32 {
			t.Fatalf("SSID %d bytes", len(ap.SSID))
		}
		if ok && (ap.FreqMHz <= 0 || ap.Seen.After(now)) {
			t.Fatalf("accepted %+v", ap)
		}
		_, _ = ParseInterface(b)
	})
}
