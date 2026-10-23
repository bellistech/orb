//go:build linux

package lldp

import (
	"errors"
	"net"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func testListener() *Listener {
	ifs := map[int]net.Interface{
		2:  {Index: 2, Name: "enp16s0", Flags: net.FlagUp, HardwareAddr: net.HardwareAddr{2, 0, 0, 0, 0, 1}},
		10: {Index: 10, Name: "veth259a985", Flags: net.FlagUp, HardwareAddr: net.HardwareAddr{2, 0, 0, 0, 0, 6}},
		11: {Index: 11, Name: "vnet0", Flags: net.FlagUp, HardwareAddr: net.HardwareAddr{2, 0, 0, 0, 0, 7}},
		12: {Index: 12, Name: "br-84fffb3566", Flags: net.FlagUp, HardwareAddr: net.HardwareAddr{2, 0, 0, 0, 0, 8}},
	}
	return &Listener{ports: map[int]port{}, lookup: func(i int) (*net.Interface, error) {
		if ifc, ok := ifs[i]; ok {
			return &ifc, nil
		}
		return nil, errors.New("no such interface")
	}}
}

// A container (or VM) on a veth (tap) can send LLDP frames to the host:
// they must neither show up as neighbors nor use up the rate budget.
func TestContainerPortsAreNotHeard(t *testing.T) {
	l := testListener()
	now := time.Unix(1000, 0)
	var heard []string
	emit := func(iface string, n Neighbor) { heard = append(heard, iface+" "+n.Name) }

	for range 2 * maxPerSecond {
		l.handle(lldpdFrame(120), 10, unix.PACKET_MULTICAST, now, emit) // veth
		l.handle(lldpdFrame(120), 11, unix.PACKET_MULTICAST, now, emit) // VM tap
		l.handle(lldpdFrame(120), 12, unix.PACKET_MULTICAST, now, emit) // bridge
		l.handle(lldpdFrame(120), 99, unix.PACKET_MULTICAST, now, emit) // gone
	}
	if len(heard) != 0 {
		t.Fatalf("heard %d frames from container plumbing: %q", len(heard), heard[:min(3, len(heard))])
	}
	l.handle(lldpdFrame(120), 2, unix.PACKET_MULTICAST, now, emit)
	if len(heard) != 1 || heard[0] != "enp16s0 east" {
		t.Fatalf("the physical port after a container flood: %q", heard)
	}
	l.handle(lldpdFrame(120), 2, unix.PACKET_OUTGOING, now, emit)
	if len(heard) != 1 {
		t.Fatalf("our own outgoing frame was heard: %q", heard)
	}
}

func TestRateLimit(t *testing.T) {
	l := testListener()
	now := time.Unix(1000, 0)
	n := 0
	emit := func(string, Neighbor) { n++ }
	for range 3 * maxPerSecond {
		l.handle(lldpdFrame(120), 2, unix.PACKET_MULTICAST, now, emit)
	}
	if n != maxPerSecond {
		t.Fatalf("decoded %d frames in one second, want %d", n, maxPerSecond)
	}
	l.handle(lldpdFrame(120), 2, unix.PACKET_MULTICAST, now.Add(time.Second), emit)
	if n != maxPerSecond+1 {
		t.Fatalf("the next second decoded nothing")
	}
}
