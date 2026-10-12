package sys

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"
)

const tcpFixture = `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:0016 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 1001 1 0000000000000000 100 0 0 10 0
   1: 0D0DA8C0:0016 6401A8C0:D431 01 00000010:00000020 02:000AF5A4 00000000     0        0 1002 4 0000000000000000 20 4 29 10 -1
   2: garbage line
   3: 0100007F:1F90 0100007F:9C40 06 00000000:00000000 03:00001234 00000000     0        0 0 3 0000000000000000
`

const tcp6Fixture = `  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000001000000:0050 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 1003 1 0000000000000000 100 0 0 10 0
   1: B80D0120000000000000000005000000:01BB B80D0120000000000000000009000000:C350 01 00000000:00000000 00:00000000 00000000  1000        0 1004 1 0000000000000000 20 4 30 10 -1
`

const udpFixture = `   sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode ref pointer drops
 2295: 00000000:0035 00000000:0000 07 00000000:00000000 00:00000000 00000000     0        0 1005 2 0000000000000000 0
`

func TestParseProcNet(t *testing.T) {
	socks := parseProcNet(tcpFixture, "tcp")
	if len(socks) != 3 {
		t.Fatalf("got %d sockets: %+v", len(socks), socks)
	}
	l := socks[0]
	if l.State != "LISTEN" || l.Local != netip.MustParseAddrPort("0.0.0.0:22") || l.Inode != 1001 {
		t.Fatalf("listen = %+v", l)
	}
	e := socks[1]
	if e.State != "ESTAB" || e.Local.String() != "192.168.13.13:22" || e.Remote.String() != "192.168.1.100:54321" || e.TxQ != 16 || e.RxQ != 32 {
		t.Fatalf("estab = %+v", e)
	}
	if socks[2].State != "TIME-WAIT" {
		t.Fatalf("time-wait = %+v", socks[2])
	}

	six := parseProcNet(tcp6Fixture, "tcp6")
	if len(six) != 2 || six[0].Local.String() != "[::1]:80" || six[1].Local.String() != "[2001:db8::5]:443" || six[1].Remote.String() != "[2001:db8::9]:50000" {
		t.Fatalf("tcp6 = %+v", six)
	}

	udp := parseProcNet(udpFixture, "udp")
	if len(udp) != 1 || udp[0].State != "UNCONN" || udp[0].Local.Port() != 53 || !udp[0].Listening() {
		t.Fatalf("udp = %+v", udp)
	}
	if e.Listening() || !l.Listening() {
		t.Fatal("Listening wrong")
	}
}

func TestParseProcNetHostile(t *testing.T) {
	for _, in := range []string{
		"",
		"header only",
		" 0: ZZZZZZZZ:0016 00000000:0000 0A 0:0 0 0 0 0 1",
		" 0: 00000000:FFFFF 00000000:0000 0A 0:0 0 0 0 0 1",
		" 0: 0000:0016 00000000:0000 0A 0:0 0 0 0 0 1",
		" 0: 00000000:0016 00000000:0000 0A",
	} {
		if got := parseProcNet("hdr\n"+in, "tcp"); len(got) != 0 {
			t.Errorf("%q parsed to %+v", in, got)
		}
	}
}

func TestSocketOwners(t *testing.T) {
	root := t.TempDir()
	mk := func(pid, comm string, fds map[string]string) {
		d := filepath.Join(root, pid, "fd")
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(root, pid, "comm"), []byte(comm+"\n"), 0o644)
		for fd, target := range fds {
			if err := os.Symlink(target, filepath.Join(d, fd)); err != nil {
				t.Fatal(err)
			}
		}
	}
	mk("100", "sshd", map[string]string{"3": "socket:[1001]", "4": "/dev/null"})
	mk("200", "nginx", map[string]string{"7": "socket:[1003]", "8": "pipe:[55]"})
	mk("self", "x", nil) // non-numeric entries are skipped
	owners := socketOwners(root, 1000)
	if o := owners[1001]; o.PID != 100 || o.Proc != "sshd" {
		t.Fatalf("1001 = %+v", o)
	}
	if o := owners[1003]; o.PID != 200 || o.Proc != "nginx" {
		t.Fatalf("1003 = %+v", o)
	}
	if len(owners) != 2 {
		t.Fatalf("owners = %+v", owners)
	}
	// The fd budget bounds the scan.
	if got := socketOwners(root, 1); len(got) > 1 {
		t.Fatalf("budget ignored: %+v", got)
	}
}

func TestParseServices(t *testing.T) {
	m := parseServices("# c\nssh\t\t22/tcp\nhttps 443/tcp  # x\nhttps 443/udp\nbogus\nx 99999/tcp\n")
	if m["tcp/22"] != "ssh" || m["udp/443"] != "https" || len(m) != 3 {
		t.Fatalf("services = %v", m)
	}
}

func TestReadSocketsLive(t *testing.T) {
	if _, err := os.Stat("/proc/net/tcp"); err != nil {
		t.Skip("no /proc/net")
	}
	for _, s := range ReadSockets(nil) {
		if s.Proto == "" || s.State == "" {
			t.Fatalf("incomplete socket %+v", s)
		}
	}
}
