package priv

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"orb/internal/lldp"
)

var east = lldp.Neighbor{Chassis: "02:00:00:00:00:03", Port: "enp1s0", TTL: 120, Name: "east", Caps: []string{"router"}, Mgmt: []string{"192.168.210.1"}}

func TestLLDPRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	if err := writeLLDP(&buf, "enp16s0", east); err != nil {
		t.Fatal(err)
	}
	tb := lldp.NewTable(true)
	err := pumpLLDP(bufio.NewReaderSize(&buf, maxLLDPLine+1), tb)
	if err == nil || !strings.Contains(err.Error(), "EOF") {
		t.Fatalf("pump ended with %v, want EOF", err)
	}
	got, on := tb.Neighbors()
	if !on || len(got) != 1 || got[0].Iface != "enp16s0" || got[0].Name != "east" {
		t.Fatalf("table = %+v %v", got, on)
	}
}

func TestReadLLDPRejects(t *testing.T) {
	for name, line := range map[string]string{
		"not json":        `east`,
		"bad interface":   `{"if":"en p16s0","n":{"chassis":"c","port":"p","ttl":5}}`,
		"long interface":  `{"if":"aaaaaaaaaaaaaaaa","n":{"chassis":"c","port":"p","ttl":5}}`,
		"no chassis":      `{"if":"enp16s0","n":{"port":"p","ttl":5}}`,
		"control in name": `{"if":"enp16s0","n":{"chassis":"c","port":"p","ttl":5,"name":"a\u001b[2Jb"}}`,
		"negative ttl":    `{"if":"enp16s0","n":{"chassis":"c","port":"p","ttl":-1}}`,
	} {
		if _, _, err := readLLDP([]byte(line)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestPumpStopsAtAnOverlongLine(t *testing.T) {
	r := bufio.NewReaderSize(strings.NewReader(strings.Repeat("x", maxLLDPLine+10)+"\n"), maxLLDPLine+1)
	if err := pumpLLDP(r, lldp.NewTable(true)); err == nil {
		t.Fatal("an overlong line did not end the pump")
	}
}

func FuzzReadLLDP(f *testing.F) {
	var buf bytes.Buffer
	_ = writeLLDP(&buf, "enp16s0", east)
	f.Add(bytes.TrimSuffix(buf.Bytes(), []byte("\n")))
	f.Fuzz(func(t *testing.T, b []byte) {
		iface, n, err := readLLDP(b)
		if err == nil && (!validIf(iface) || n.Check() != nil) {
			t.Fatalf("accepted %q %+v", iface, n)
		}
	})
}
