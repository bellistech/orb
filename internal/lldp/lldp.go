// Package lldp decodes IEEE 802.1AB LLDP frames and keeps the neighbors
// heard, each until its announced time to live runs out.
//
// Frames come off the wire from anything on the link: every length is
// checked, every string reduced to printable ASCII and capped.
package lldp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"

	"orb/internal/sys"
)

// EtherType and Group are LLDP's ethertype and its nearest-bridge
// destination address (01:80:c2:00:00:0e).
const EtherType = 0x88cc

var Group = [6]byte{0x01, 0x80, 0xc2, 0x00, 0x00, 0x0e}

// Limits for what one neighbor may carry.
const (
	maxField   = 128 // characters per string shown
	maxDesc    = 255
	maxMgmt    = 4
	maxEntries = 64
)

// Neighbor is one decoded LLDPDU.
type Neighbor struct {
	Chassis  string   `json:"chassis"`
	Port     string   `json:"port"`
	TTL      int      `json:"ttl"`
	PortDesc string   `json:"port_desc,omitempty"`
	Name     string   `json:"name,omitempty"`
	Desc     string   `json:"desc,omitempty"`
	Caps     []string `json:"caps,omitempty"`
	Mgmt     []string `json:"mgmt,omitempty"`
}

var errFrame = errors.New("lldp: malformed frame")

// printable keeps printable ASCII, turns anything else into '?', trims and
// caps the result at n characters.
func printable(b []byte, n int) string {
	var sb strings.Builder
	for _, c := range b {
		if sb.Len() >= n {
			break
		}
		if c >= 0x20 && c < 0x7f {
			sb.WriteByte(c)
		} else if c == '\t' || c == '\n' || c == '\r' {
			sb.WriteByte(' ')
		} else {
			sb.WriteByte('?')
		}
	}
	return strings.TrimSpace(sb.String())
}

func mac(b []byte) string {
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", b[0], b[1], b[2], b[3], b[4], b[5])
}

// netAddr decodes an IANA address-family-prefixed address (1 IPv4, 2 IPv6).
func netAddr(b []byte) (string, bool) {
	if len(b) < 1 {
		return "", false
	}
	switch {
	case b[0] == 1 && len(b) == 5, b[0] == 2 && len(b) == 17:
		a, _ := netip.AddrFromSlice(b[1:])
		return a.String(), true
	}
	return "", false
}

// idString renders a chassis or port ID by its subtype.
func idString(sub byte, v []byte, macSub, addrSub byte) string {
	switch {
	case sub == macSub && len(v) == 6:
		return mac(v)
	case sub == addrSub:
		if a, ok := netAddr(v); ok {
			return a
		}
	}
	return printable(v, maxField)
}

var capNames = []string{"other", "repeater", "bridge", "wlan-ap", "router", "phone", "docsis", "station", "c-vlan", "s-vlan", "tpmr"}

// Decode parses one LLDPDU (the frame payload after the Ethernet header).
// The first three TLVs must be chassis ID, port ID and TTL (802.1AB 8.2).
func Decode(b []byte) (Neighbor, error) {
	var n Neighbor
	seen := 0
	for len(b) >= 2 {
		h := binary.BigEndian.Uint16(b)
		typ, l := int(h>>9), int(h&0x1ff)
		b = b[2:]
		if l > len(b) {
			return Neighbor{}, errFrame
		}
		v := b[:l]
		b = b[l:]
		if seen < 3 && typ != seen+1 {
			return Neighbor{}, errFrame
		}
		switch typ {
		case 0: // end of LLDPDU
			if seen < 3 {
				return Neighbor{}, errFrame
			}
			return n, n.identified()
		case 1: // chassis ID
			if l < 2 {
				return Neighbor{}, errFrame
			}
			n.Chassis = idString(v[0], v[1:], 4, 5)
		case 2: // port ID
			if l < 2 {
				return Neighbor{}, errFrame
			}
			n.Port = idString(v[0], v[1:], 3, 4)
		case 3: // TTL
			if l < 2 {
				return Neighbor{}, errFrame
			}
			n.TTL = int(binary.BigEndian.Uint16(v))
		case 4:
			n.PortDesc = printable(v, maxField)
		case 5:
			n.Name = printable(v, maxField)
		case 6:
			n.Desc = printable(v, maxDesc)
		case 7: // system capabilities: supported, then enabled
			if l == 4 {
				en := binary.BigEndian.Uint16(v[2:])
				for i, c := range capNames {
					if en&(1<<i) != 0 {
						n.Caps = append(n.Caps, c)
					}
				}
			}
		case 8: // management address: length, subtype + address, ...
			if l >= 2 && int(v[0]) >= 2 && int(v[0])+1 <= l && len(n.Mgmt) < maxMgmt {
				if a, ok := netAddr(v[1 : 1+int(v[0])]); ok {
					n.Mgmt = append(n.Mgmt, a)
				}
			}
		}
		seen++
	}
	if seen < 3 {
		return Neighbor{}, errFrame
	}
	return n, n.identified() // no End TLV: tolerated, as lldpd does
}

// identified: an ID that sanitizes to nothing (all blanks) names no one.
func (n Neighbor) identified() error {
	if n.Chassis == "" || n.Port == "" {
		return errFrame
	}
	return nil
}

// Check validates a neighbor that came over the helper pipe: the main
// process trusts nothing the helper sends.
func (n Neighbor) Check() error {
	if n.Chassis == "" || n.Port == "" || n.TTL < 0 || n.TTL > 65535 || len(n.Caps) > len(capNames) || len(n.Mgmt) > maxMgmt {
		return errFrame
	}
	for _, s := range append([]string{n.Chassis, n.Port, n.PortDesc, n.Name, n.Desc}, append(n.Caps, n.Mgmt...)...) {
		if len(s) > maxDesc || printable([]byte(s), maxDesc) != s {
			return errFrame
		}
	}
	return nil
}

type entry struct {
	n     Neighbor
	iface string
	heard time.Time
}

// Table holds the neighbors heard, keyed by interface, chassis and port.
// Safe for concurrent use.
type Table struct {
	mu      sync.Mutex
	entries map[string]entry
	on      bool
	now     func() time.Time
}

// NewTable returns an empty table; on says whether a listener feeds it.
func NewTable(on bool) *Table {
	return &Table{entries: map[string]entry{}, on: on, now: time.Now}
}

// Heard records n from iface. TTL 0 is a shutdown: the neighbor leaves.
func (t *Table) Heard(iface string, n Neighbor) {
	t.mu.Lock()
	defer t.mu.Unlock()
	k := iface + "\x00" + n.Chassis + "\x00" + n.Port
	if n.TTL == 0 {
		delete(t.entries, k)
		return
	}
	if _, ok := t.entries[k]; !ok && len(t.entries) >= maxEntries {
		return
	}
	t.entries[k] = entry{n: n, iface: iface, heard: t.now()}
}

// Neighbors returns the live neighbors (expired ones are dropped) and
// whether a listener is running. It is the sampler's LLDP hook.
func (t *Table) Neighbors() ([]sys.LLDPNeighbor, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	var out []sys.LLDPNeighbor
	for k, e := range t.entries {
		age := now.Sub(e.heard)
		if age > time.Duration(e.n.TTL)*time.Second {
			delete(t.entries, k)
			continue
		}
		out = append(out, sys.LLDPNeighbor{
			Iface: e.iface, Chassis: e.n.Chassis, Port: e.n.Port, PortDesc: e.n.PortDesc,
			Name: e.n.Name, Desc: e.n.Desc, Caps: e.n.Caps, Mgmt: e.n.Mgmt,
			TTL: e.n.TTL, Age: int(age / time.Second),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Iface != out[j].Iface {
			return out[i].Iface < out[j].Iface
		}
		return out[i].Name < out[j].Name
	})
	return out, t.on
}
