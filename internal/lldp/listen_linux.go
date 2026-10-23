//go:build linux

package lldp

import (
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// Listener receives LLDP frames on every interface.
type Listener struct {
	fd     int
	joined map[int]bool
}

func htons(v uint16) uint16 { return v<<8 | v>>8 }

// Open creates the packet socket for LLDP's ethertype on all interfaces.
// It needs CAP_NET_RAW; reading from it afterwards needs nothing.
func Open() (*Listener, error) {
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, int(htons(EtherType)))
	if err != nil {
		return nil, err
	}
	tv := unix.Timeval{Sec: 1}
	_ = unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv)
	return &Listener{fd: fd, joined: map[int]bool{}}, nil
}

// carriesLLDP: physical ports and their bonds/VLANs, not loopback or
// container plumbing.
func carriesLLDP(ifc net.Interface) bool {
	if ifc.Flags&net.FlagLoopback != 0 || ifc.Flags&net.FlagUp == 0 || len(ifc.HardwareAddr) != 6 {
		return false
	}
	for _, p := range []string{"veth", "docker", "br-", "virbr", "lxdbr", "cni"} {
		if strings.HasPrefix(ifc.Name, p) {
			return false
		}
	}
	return true
}

// Join adds LLDP's multicast group on every interface that carries it and
// was not joined yet, so NICs that filter multicast still pass the frames.
// Changing an open socket's memberships needs no privilege, so Join keeps
// working after the drop; call it again to cover interfaces that appear.
func (l *Listener) Join() {
	ifs, err := net.Interfaces()
	if err != nil {
		return
	}
	for _, ifc := range ifs {
		if l.joined[ifc.Index] || !carriesLLDP(ifc) {
			continue
		}
		mr := unix.PacketMreq{Ifindex: int32(ifc.Index), Type: unix.PACKET_MR_MULTICAST, Alen: 6}
		copy(mr.Address[:], Group[:])
		if unix.SetsockoptPacketMreq(l.fd, unix.SOL_PACKET, unix.PACKET_ADD_MEMBERSHIP, &mr) == nil {
			l.joined[ifc.Index] = true
		}
	}
}

// maxPerSecond bounds how many frames are decoded each second; LLDP sends
// one per neighbor every 30 s, so anything near this is a flood.
const maxPerSecond = 64

// Run decodes frames until the socket fails, calling emit for each valid
// neighbor. It rejoins every minute for new interfaces.
func (l *Listener) Run(emit func(iface string, n Neighbor)) error {
	buf := make([]byte, 1600)
	names := map[int]string{}
	window, count := time.Now(), 0
	lastJoin := time.Now()
	for {
		if time.Since(lastJoin) > time.Minute {
			l.Join()
			lastJoin = time.Now()
		}
		n, from, err := unix.Recvfrom(l.fd, buf, 0)
		if err == unix.EAGAIN || err == unix.EINTR {
			continue
		}
		if err != nil {
			return err
		}
		ll, ok := from.(*unix.SockaddrLinklayer)
		if !ok || ll.Pkttype == unix.PACKET_OUTGOING {
			continue
		}
		if time.Since(window) >= time.Second {
			window, count = time.Now(), 0
		}
		if count++; count > maxPerSecond {
			continue
		}
		nb, err := Decode(buf[:n])
		if err != nil {
			continue
		}
		name, ok := names[ll.Ifindex]
		if !ok {
			if ifc, err := net.InterfaceByIndex(ll.Ifindex); err == nil {
				name = ifc.Name
			} else {
				name = "#" + strconv.Itoa(ll.Ifindex)
			}
			names[ll.Ifindex] = name
		}
		emit(name, nb)
	}
}

// Close closes the socket.
func (l *Listener) Close() error { return unix.Close(l.fd) }
