//go:build linux

package radio

import (
	"encoding/binary"
	"errors"
	"syscall"
	"time"
)

// conn is one generic netlink socket.
type conn struct {
	fd  int
	seq uint32
}

func open() (*conn, error) {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW|syscall.SOCK_CLOEXEC, syscall.NETLINK_GENERIC)
	if err != nil {
		return nil, err
	}
	if err := syscall.Bind(fd, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}); err != nil {
		syscall.Close(fd)
		return nil, err
	}
	tv := syscall.Timeval{Sec: 2}
	_ = syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)
	return &conn{fd: fd}, nil
}

func (c *conn) close() { syscall.Close(c.fd) }

// maxDump bounds what one dump may return.
const maxDump = 16 << 20

// do sends one generic netlink request and returns the payloads (after
// the genetlink header) of the replies of type want. A dump ends at
// NLMSG_DONE; a plain request at its acknowledgement.
func (c *conn) do(typ uint16, flags uint16, cmd uint8, attrs []byte, want uint16) ([][]byte, error) {
	c.seq++
	req := make([]byte, syscall.NLMSG_HDRLEN+GenlHdrLen, syscall.NLMSG_HDRLEN+GenlHdrLen+len(attrs))
	req = append(req, attrs...)
	binary.NativeEndian.PutUint32(req[0:4], uint32(len(req)))
	binary.NativeEndian.PutUint16(req[4:6], typ)
	binary.NativeEndian.PutUint16(req[6:8], syscall.NLM_F_REQUEST|syscall.NLM_F_ACK|flags)
	binary.NativeEndian.PutUint32(req[8:12], c.seq)
	req[syscall.NLMSG_HDRLEN] = cmd
	req[syscall.NLMSG_HDRLEN+1] = 1 // version
	if err := syscall.Sendto(c.fd, req, 0, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}); err != nil {
		return nil, err
	}
	var out [][]byte
	total := 0
	buf := make([]byte, 128<<10)
	for range 8192 {
		n, _, err := syscall.Recvfrom(c.fd, buf, 0)
		if err != nil {
			return out, err
		}
		msgs, err := syscall.ParseNetlinkMessage(buf[:n])
		if err != nil {
			return out, err
		}
		for _, m := range msgs {
			if m.Header.Seq != c.seq {
				continue
			}
			switch m.Header.Type {
			case syscall.NLMSG_DONE:
				return out, nil
			case syscall.NLMSG_ERROR:
				if len(m.Data) < 4 {
					return out, errors.New("short netlink error")
				}
				if e := -int32(binary.NativeEndian.Uint32(m.Data[0:4])); e != 0 {
					return out, syscall.Errno(e)
				}
				if flags&syscall.NLM_F_DUMP == 0 {
					return out, nil // the acknowledgement of a plain request
				}
			case want:
				if len(m.Data) < GenlHdrLen {
					continue
				}
				total += len(m.Data)
				if total > maxDump {
					return out, errors.New("netlink dump too large")
				}
				out = append(out, append([]byte(nil), m.Data[GenlHdrLen:]...))
			}
		}
	}
	return out, errors.New("netlink dump did not end")
}

// family resolves nl80211's generic netlink id.
func (c *conn) family() (uint16, error) {
	rs, err := c.do(GenlIDCtrl, 0, CtrlCmdGetFamily, attrBytes(CtrlAttrFamilyName, []byte("nl80211\x00")), GenlIDCtrl)
	if err != nil {
		return 0, err
	}
	var id uint16
	for _, r := range rs {
		Attrs(r, func(t uint16, v []byte) {
			if t == CtrlAttrFamilyID && len(v) >= 2 {
				id = binary.NativeEndian.Uint16(v)
			}
		})
	}
	if id == 0 {
		return 0, errors.New("no nl80211 (no Wi-Fi driver loaded)")
	}
	return id, nil
}

func attrBytes(t uint16, v []byte) []byte {
	b := make([]byte, 4, 4+len(v)+3)
	binary.NativeEndian.PutUint16(b[0:2], uint16(4+len(v)))
	binary.NativeEndian.PutUint16(b[2:4], t)
	b = append(b, v...)
	for len(b)%4 != 0 {
		b = append(b, 0)
	}
	return b
}

func u32attr(t uint16, x uint32) []byte {
	v := make([]byte, 4)
	binary.NativeEndian.PutUint32(v, x)
	return attrBytes(t, v)
}

// Interfaces lists the host's Wi-Fi interfaces. It needs no privilege.
func Interfaces() ([]Iface, error) {
	c, err := open()
	if err != nil {
		return nil, err
	}
	defer c.close()
	fam, err := c.family()
	if err != nil {
		return nil, err
	}
	rs, err := c.do(fam, syscall.NLM_F_DUMP, CmdGetInterface, nil, fam)
	var out []Iface
	for _, r := range rs {
		if i, ok := ParseInterface(r); ok {
			out = append(out, i)
		}
	}
	return out, err
}

// Heard returns the networks the kernel has heard on one interface (its
// scan results, refreshed by any scan, ours or a supplicant's). It needs
// no privilege and sends nothing on the air.
func Heard(ifindex int, iface string, now time.Time) ([]AP, error) {
	c, err := open()
	if err != nil {
		return nil, err
	}
	defer c.close()
	fam, err := c.family()
	if err != nil {
		return nil, err
	}
	rs, err := c.do(fam, syscall.NLM_F_DUMP, CmdGetScan, u32attr(AttrIfindex, uint32(ifindex)), fam)
	var out []AP
	for _, r := range rs {
		if ap, ok := ParseBSS(r, now); ok {
			ap.Iface = iface
			out = append(out, ap)
		}
	}
	return out, err
}

// TriggerPassive asks one interface to listen on every channel it may use.
// The request names no SSID, so the scan is passive: the radio sends no
// probe requests, it only hears beacons. It needs CAP_NET_ADMIN and an
// interface that is up; it never brings one up. A radio in use briefly
// leaves its channel while it listens.
func TriggerPassive(ifindex int) error {
	c, err := open()
	if err != nil {
		return err
	}
	defer c.close()
	fam, err := c.family()
	if err != nil {
		return err
	}
	_, err = c.do(fam, 0, CmdTriggerScan, u32attr(AttrIfindex, uint32(ifindex)), fam)
	return err
}
