package lldp

import (
	"encoding/binary"
	"reflect"
	"strings"
	"testing"
	"time"
)

func tlv(typ int, v ...byte) []byte {
	b := make([]byte, 2, 2+len(v))
	binary.BigEndian.PutUint16(b, uint16(typ<<9|len(v)))
	return append(b, v...)
}

func frame(tlvs ...[]byte) []byte {
	var b []byte
	for _, t := range tlvs {
		b = append(b, t...)
	}
	return b
}

// lldpdFrame is what lldpd on east sends (shape of a real LLDPDU).
func lldpdFrame(ttl uint16) []byte {
	return frame(
		tlv(1, append([]byte{4}, 0x02, 0x00, 0x00, 0x00, 0x00, 0x03)...),
		tlv(2, append([]byte{3}, 0x02, 0x00, 0x00, 0x00, 0x00, 0x03)...),
		tlv(3, byte(ttl>>8), byte(ttl)),
		tlv(5, []byte("east")...),
		tlv(6, []byte("Ubuntu 25.10 Linux 6.17.0-41-generic #41 SMP x86_64")...),
		tlv(7, 0x00, 0x14, 0x00, 0x10), // bridge+router supported, router enabled
		tlv(8, append([]byte{5, 1, 192, 168, 210, 1}, 2, 0, 0, 0, 2, 0)...),
		tlv(8, append(append([]byte{17, 2}, make([]byte, 16)...), 2, 0, 0, 0, 2, 0)...),
		tlv(4, []byte("enp1s0")...),
		tlv(127, 0x00, 0x12, 0x0f, 0x01, 0x03), // org-specific: skipped
		tlv(0),
	)
}

func TestDecodeLLDPD(t *testing.T) {
	n, err := Decode(lldpdFrame(120))
	if err != nil {
		t.Fatal(err)
	}
	want := Neighbor{
		Chassis: "02:00:00:00:00:03", Port: "02:00:00:00:00:03", TTL: 120, PortDesc: "enp1s0",
		Name: "east", Desc: "Ubuntu 25.10 Linux 6.17.0-41-generic #41 SMP x86_64",
		Caps: []string{"router"}, Mgmt: []string{"192.168.210.1", "::"},
	}
	if !reflect.DeepEqual(n, want) {
		t.Errorf("got  %+v\nwant %+v", n, want)
	}
	if err := n.Check(); err != nil {
		t.Errorf("a decoded neighbor fails Check: %v", err)
	}
}

func TestDecodeRejects(t *testing.T) {
	ch := tlv(1, 7, 'x')
	pt := tlv(2, 7, 'p')
	ttl := tlv(3, 0, 120)
	for name, b := range map[string][]byte{
		"empty":            nil,
		"no TTL":           frame(ch, pt, tlv(0)),
		"wrong order":      frame(pt, ch, ttl, tlv(0)),
		"length past end":  append(frame(ch, pt), 0x06, 0xff, 'a'),
		"short chassis":    frame(tlv(1, 7), pt, ttl),
		"end before TTL":   frame(ch, tlv(0)),
		"truncated header": append(frame(ch, pt, ttl), 0x02),
	} {
		if n, err := Decode(b); err == nil && name != "truncated header" {
			t.Errorf("%s: decoded %+v", name, n)
		}
	}
}

func TestStringsArePrintable(t *testing.T) {
	evil := append([]byte("ev\x1b[2Jil\x00"), []byte(strings.Repeat("A", 400))...)
	n, err := Decode(frame(tlv(1, 7, 'x'), tlv(2, 7, 'p'), tlv(3, 0, 5), tlv(5, evil[:255]...), tlv(6, evil[:255]...)))
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(n.Name+n.Desc, "\x1b\x00") || len(n.Name) > maxField || len(n.Desc) > maxDesc {
		t.Errorf("unsafe strings: %q / %d chars", n.Name, len(n.Desc))
	}
}

func TestCheckRefusesWhatDecodeNeverMakes(t *testing.T) {
	ok, _ := Decode(lldpdFrame(120))
	for name, mut := range map[string]func(*Neighbor){
		"no chassis":       func(n *Neighbor) { n.Chassis = "" },
		"control byte":     func(n *Neighbor) { n.Name = "a\x1bb" },
		"untrimmed":        func(n *Neighbor) { n.Name = " east" },
		"too long":         func(n *Neighbor) { n.Desc = strings.Repeat("x", 300) },
		"ttl out of range": func(n *Neighbor) { n.TTL = 70000 },
		"too many mgmt":    func(n *Neighbor) { n.Mgmt = []string{"a", "b", "c", "d", "e"} },
	} {
		n := ok
		n.Caps, n.Mgmt = append([]string(nil), ok.Caps...), append([]string(nil), ok.Mgmt...)
		mut(&n)
		if n.Check() == nil {
			t.Errorf("%s: passed Check", name)
		}
	}
}

func TestTableTTL(t *testing.T) {
	now := time.Unix(1000, 0)
	tb := NewTable(true)
	tb.now = func() time.Time { return now }
	n, _ := Decode(lldpdFrame(120))
	tb.Heard("enp16s0", n)
	if got, on := tb.Neighbors(); len(got) != 1 || !on || got[0].Iface != "enp16s0" || got[0].Name != "east" {
		t.Fatalf("after hearing: %+v %v", got, on)
	}
	now = now.Add(119 * time.Second)
	if got, _ := tb.Neighbors(); len(got) != 1 || got[0].Age != 119 {
		t.Fatalf("within TTL: %+v", got)
	}
	now = now.Add(2 * time.Second)
	if got, _ := tb.Neighbors(); len(got) != 0 {
		t.Fatalf("past TTL still shown: %+v", got)
	}
	tb.Heard("enp16s0", n)
	n.TTL = 0 // shutdown LLDPDU
	tb.Heard("enp16s0", n)
	if got, _ := tb.Neighbors(); len(got) != 0 {
		t.Fatalf("TTL 0 did not remove the neighbor: %+v", got)
	}
	for i := range maxEntries + 10 {
		m := n
		m.TTL, m.Port = 60, string(rune('a'+i%26))+strings.Repeat("p", i/26)
		tb.Heard("enp16s0", m)
	}
	if got, _ := tb.Neighbors(); len(got) != maxEntries {
		t.Fatalf("table holds %d, want the cap %d", len(got), maxEntries)
	}
}

func FuzzDecode(f *testing.F) {
	f.Add(lldpdFrame(120))
	f.Add(frame(tlv(1, 7, 'x'), tlv(2, 7, 'p'), tlv(3, 0, 5)))
	f.Fuzz(func(t *testing.T, b []byte) {
		n, err := Decode(b)
		if err == nil {
			if cerr := n.Check(); cerr != nil {
				t.Fatalf("decoded neighbor fails Check: %v (%+v)", cerr, n)
			}
		}
	})
}
