//go:build linux

package priv

import (
	"os"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"orb/internal/lldp"
)

// The LLDP helper's life, as root of a user namespace with its own network
// namespace (make test-priv): open the packet socket, drop to nobody with
// nothing, and the socket still works while a new one is refused.
func TestLLDPHelperDrop(t *testing.T) {
	needRoot(t)
	l, err := lldp.Open()
	if err != nil {
		t.Fatalf("open as root: %v", err)
	}
	defer l.Close()
	if err := Drop(nobody); err != nil {
		t.Fatalf("Drop: %v", err)
	}
	status, _ := os.ReadFile("/proc/thread-self/status")
	for _, line := range strings.Split(string(status), "\n") {
		if strings.HasPrefix(line, "CapEff:") || strings.HasPrefix(line, "CapPrm:") || strings.HasPrefix(line, "CapBnd:") {
			if f := strings.Fields(line); len(f) != 2 || strings.Trim(f[1], "0") != "" {
				t.Errorf("capabilities left: %s", line)
			}
		}
	}
	if fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_DGRAM, int(0xcc88)); err == nil {
		unix.Close(fd)
		t.Fatal("nobody could open a new packet socket")
	}
	l.Join() // memberships on the open socket need no privilege
	if err := l.Close(); err != nil {
		t.Fatalf("the open socket is unusable after the drop: %v", err)
	}
}
