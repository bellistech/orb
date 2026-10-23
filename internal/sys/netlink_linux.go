package sys

import (
	"encoding/binary"
	"sort"
	"syscall"
)

// dump returns the bodies of one rtnetlink dump, or nil. The request
// carries a full ndmsg/rtmsg-sized header with only the family set:
// syscall.NetlinkRIB sends a 1-byte rtgenmsg, which the bridge FDB dump
// ignores.
func dump(typ, family int, want uint16) [][]byte {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW|syscall.SOCK_CLOEXEC, syscall.NETLINK_ROUTE)
	if err != nil {
		return nil
	}
	defer syscall.Close(fd)
	if err := syscall.Bind(fd, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}); err != nil {
		return nil
	}
	tv := syscall.Timeval{Sec: 1}
	_ = syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)
	req := make([]byte, syscall.NLMSG_HDRLEN+ndmsgLen)
	binary.NativeEndian.PutUint32(req[0:4], uint32(len(req)))
	binary.NativeEndian.PutUint16(req[4:6], uint16(typ))
	binary.NativeEndian.PutUint16(req[6:8], syscall.NLM_F_REQUEST|syscall.NLM_F_DUMP)
	binary.NativeEndian.PutUint32(req[8:12], 1) // seq
	req[syscall.NLMSG_HDRLEN] = byte(family)
	if err := syscall.Sendto(fd, req, 0, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}); err != nil {
		return nil
	}
	var out [][]byte
	buf := make([]byte, 64<<10)
	for range 4096 { // bounded: a dump of a few thousand messages at most
		n, _, err := syscall.Recvfrom(fd, buf, 0)
		if err != nil || n < syscall.NLMSG_HDRLEN {
			return out
		}
		msgs, err := syscall.ParseNetlinkMessage(buf[:n])
		if err != nil {
			return out
		}
		for _, m := range msgs {
			switch m.Header.Type {
			case syscall.NLMSG_DONE, syscall.NLMSG_ERROR:
				return out
			case want:
				out = append(out, append([]byte(nil), m.Data...))
			}
		}
	}
	return out
}

// readL2L3 dumps neighbors, routes and the bridges' learned MACs. Dumps
// need no privilege.
func readL2L3(ifname func(int) string) (ns []Neighbor, rs []Route, fdb []FDBEntry) {
	for _, b := range dump(syscall.RTM_GETNEIGH, syscall.AF_UNSPEC, syscall.RTM_NEWNEIGH) {
		if n, ok := parseNeigh(b, ifname); ok {
			ns = append(ns, n)
		}
	}
	for _, b := range dump(syscall.RTM_GETROUTE, syscall.AF_UNSPEC, syscall.RTM_NEWROUTE) {
		if r, ok := parseRoute(b, ifname); ok {
			rs = append(rs, r)
		}
	}
	for _, b := range dump(syscall.RTM_GETNEIGH, afBridge, syscall.RTM_NEWNEIGH) {
		if e, ok := parseFDB(b, ifname); ok {
			fdb = append(fdb, e)
		}
	}
	sortNeigh(ns)
	sortRoutes(rs)
	sort.SliceStable(fdb, func(i, j int) bool {
		if fdb[i].Bridge != fdb[j].Bridge {
			return fdb[i].Bridge < fdb[j].Bridge
		}
		return fdb[i].Port < fdb[j].Port
	})
	return ns, rs, fdb
}
