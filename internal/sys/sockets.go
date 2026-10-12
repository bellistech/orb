package sys

import (
	"encoding/hex"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Socket is one entry from /proc/net/{tcp,tcp6,udp,udp6}.
type Socket struct {
	Proto         string // tcp, tcp6, udp, udp6
	Local, Remote netip.AddrPort
	State         string // ss-style: LISTEN, ESTAB, TIME-WAIT, UNCONN...
	Inode         uint64
	TxQ, RxQ      uint64
	PID           int    // 0 when the owner is not visible to us
	Proc          string // owning process name, "" when unknown
}

// Listening reports a TCP listener or an unconnected UDP socket.
func (s Socket) Listening() bool {
	return s.State == "LISTEN" || s.State == "UNCONN"
}

// tcpStates maps the kernel's hex state to ss's names.
var tcpStates = map[string]string{
	"01": "ESTAB", "02": "SYN-SENT", "03": "SYN-RECV", "04": "FIN-WAIT-1",
	"05": "FIN-WAIT-2", "06": "TIME-WAIT", "07": "CLOSE", "08": "CLOSE-WAIT",
	"09": "LAST-ACK", "0A": "LISTEN", "0B": "CLOSING",
}

// maxSockets bounds how many sockets one read keeps.
const maxSockets = 20000

// parseProcNet parses one /proc/net table. Malformed lines are skipped.
func parseProcNet(table, proto string) []Socket {
	var out []Socket
	lines := strings.Split(table, "\n")
	for _, line := range lines[min(1, len(lines)):] { // first line is a header
		f := strings.Fields(line)
		if len(f) < 10 || len(out) >= maxSockets {
			continue
		}
		local, ok1 := parseHexAddrPort(f[1])
		remote, ok2 := parseHexAddrPort(f[2])
		state, ok3 := tcpStates[strings.ToUpper(f[3])]
		if !ok1 || !ok2 || !ok3 {
			continue
		}
		if strings.HasPrefix(proto, "udp") {
			// UDP has no connection states; ss shows 07 as UNCONN.
			if state == "CLOSE" {
				state = "UNCONN"
			}
		}
		tx, rx, _ := strings.Cut(f[4], ":")
		inode, err := strconv.ParseUint(f[9], 10, 64)
		if err != nil {
			continue
		}
		txq, _ := strconv.ParseUint(tx, 16, 64)
		rxq, _ := strconv.ParseUint(rx, 16, 64)
		out = append(out, Socket{Proto: proto, Local: local, Remote: remote, State: state, Inode: inode, TxQ: txq, RxQ: rxq})
	}
	return out
}

// parseHexAddrPort decodes "0100007F:0016" (IPv4) or a 32-hex-digit IPv6
// address. The kernel prints each 32-bit word of the address in host byte
// order (little-endian here); the port is plain big-endian hex.
func parseHexAddrPort(s string) (netip.AddrPort, bool) {
	a, p, ok := strings.Cut(s, ":")
	if !ok || len(p) != 4 || (len(a) != 8 && len(a) != 32) {
		return netip.AddrPort{}, false
	}
	port, err := strconv.ParseUint(p, 16, 16)
	if err != nil {
		return netip.AddrPort{}, false
	}
	raw, err := hex.DecodeString(a)
	if err != nil {
		return netip.AddrPort{}, false
	}
	for w := 0; w < len(raw); w += 4 {
		raw[w], raw[w+1], raw[w+2], raw[w+3] = raw[w+3], raw[w+2], raw[w+1], raw[w]
	}
	addr, ok := netip.AddrFromSlice(raw)
	if !ok {
		return netip.AddrPort{}, false
	}
	return netip.AddrPortFrom(addr.Unmap(), uint16(port)), true
}

// Owner is the process holding a socket.
type Owner struct {
	PID  int
	Proc string
}

// OwnerFunc maps socket inodes to their owners.
type OwnerFunc func() map[uint64]Owner

// LocalOwners scans /proc directly: every process when privileged, only
// our own otherwise.
func LocalOwners() map[uint64]Owner { return socketOwners("/proc", MaxFDScan) }

// socketOwners maps socket inodes to the processes holding them by reading
// the fd symlinks under procRoot. Only processes we may inspect are found.
// maxFDs bounds the work.
func socketOwners(procRoot string, maxFDs int) map[uint64]Owner {
	out := map[uint64]Owner{}
	ents, err := os.ReadDir(procRoot)
	if err != nil {
		return out
	}
	seen := 0
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		dir := filepath.Join(procRoot, e.Name())
		fds, err := os.ReadDir(filepath.Join(dir, "fd"))
		if err != nil {
			continue
		}
		var comm string
		for _, fd := range fds {
			if seen >= maxFDs {
				return out
			}
			seen++
			target, err := os.Readlink(filepath.Join(dir, "fd", fd.Name()))
			if err != nil || !strings.HasPrefix(target, "socket:[") || !strings.HasSuffix(target, "]") {
				continue
			}
			inode, err := strconv.ParseUint(target[8:len(target)-1], 10, 64)
			if err != nil {
				continue
			}
			if comm == "" {
				comm = readString(filepath.Join(dir, "comm"))
			}
			if _, dup := out[inode]; !dup {
				out[inode] = Owner{PID: pid, Proc: comm}
			}
		}
	}
	return out
}

// MaxFDScan bounds the per-read fd walk; a busy box has a few thousand.
const MaxFDScan = 200000

// ReadSockets reads every TCP and UDP socket and, where owners can see it,
// its owning process. Listeners first by port, then the rest. A nil owners
// means LocalOwners.
func ReadSockets(owners OwnerFunc) []Socket {
	var all []Socket
	for _, proto := range []string{"tcp", "tcp6", "udp", "udp6"} {
		all = append(all, parseProcNet(readString("/proc/net/"+proto), proto)...)
	}
	if owners == nil {
		owners = LocalOwners
	}
	om := owners()
	for i := range all {
		if o, ok := om[all[i].Inode]; ok && all[i].Inode != 0 {
			all[i].PID, all[i].Proc = o.PID, o.Proc
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		a, b := all[i], all[j]
		if a.Listening() != b.Listening() {
			return a.Listening()
		}
		if a.Listening() {
			return a.Local.Port() < b.Local.Port() || a.Local.Port() == b.Local.Port() && a.Proto < b.Proto
		}
		return a.Proc < b.Proc || a.Proc == b.Proc && a.Remote.String() < b.Remote.String()
	})
	return all
}

// parseServices reads /etc/services into "proto/port" -> name.
func parseServices(text string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		port, proto, ok := strings.Cut(f[1], "/")
		if n, err := strconv.ParseUint(port, 10, 16); !ok || err != nil || n == 0 {
			continue
		}
		key := proto + "/" + port
		if _, dup := out[key]; !dup {
			out[key] = f[0]
		}
	}
	return out
}

var services = sync.OnceValue(func() map[string]string { return parseServices(readString("/etc/services")) })

// Service names a well-known port from /etc/services, or "".
func Service(proto string, port uint16) string {
	return services()[strings.TrimSuffix(proto, "6")+"/"+strconv.Itoa(int(port))]
}
