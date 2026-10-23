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

	// ports caches, per interface index, its name and whether frames
	// from it count (carriesLLDP); refreshed with every Join.
	ports  map[int]port
	lookup func(index int) (*net.Interface, error)

	window time.Time // start of the current rate window
	count  int       // frames decoded in it
}

type port struct {
	name string
	ok   bool
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
	return &Listener{fd: fd, joined: map[int]bool{}, ports: map[int]port{}, lookup: net.InterfaceByIndex}, nil
}

// carriesLLDP: physical ports and their bonds/VLANs, not loopback or
// container and VM plumbing (veths, bridges, taps), where whoever runs in
// the container or VM chooses what arrives.
func carriesLLDP(ifc net.Interface) bool {
	if ifc.Flags&net.FlagLoopback != 0 || ifc.Flags&net.FlagUp == 0 || len(ifc.HardwareAddr) != 6 {
		return false
	}
	for _, p := range []string{"veth", "docker", "br-", "virbr", "lxdbr", "cni", "vnet", "tap", "macvtap"} {
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
	clear(l.ports) // indexes are reused; names change
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
		if ll, ok := from.(*unix.SockaddrLinklayer); ok {
			l.handle(buf[:n], ll.Ifindex, ll.Pkttype, time.Now(), emit)
		}
	}
}

// handle decodes one received frame. The packet socket hears every
// interface, so frames from ports that do not carry LLDP (a container's
// veth, a VM's tap) are dropped before they count against the rate limit:
// a container must not be able to plant neighbors or starve real ones.
func (l *Listener) handle(frame []byte, ifindex int, pkttype uint8, now time.Time, emit func(string, Neighbor)) {
	if pkttype == unix.PACKET_OUTGOING {
		return
	}
	p, ok := l.ports[ifindex]
	if !ok {
		p = port{name: "#" + strconv.Itoa(ifindex)}
		if ifc, err := l.lookup(ifindex); err == nil {
			p = port{name: ifc.Name, ok: carriesLLDP(*ifc)}
		}
		l.ports[ifindex] = p
	}
	if !p.ok {
		return
	}
	if now.Sub(l.window) >= time.Second {
		l.window, l.count = now, 0
	}
	if l.count++; l.count > maxPerSecond {
		return
	}
	if nb, err := Decode(frame); err == nil {
		emit(p.name, nb)
	}
}

// Close closes the socket.
func (l *Listener) Close() error { return unix.Close(l.fd) }
