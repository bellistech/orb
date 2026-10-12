package priv

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"testing"

	"orb/internal/sys"
)

func TestOwnersRoundTrip(t *testing.T) {
	in := map[uint64]sys.Owner{
		1001: {PID: 100, Proc: "sshd"},
		1002: {PID: 200, Proc: "evil\x1b[2J name with spaces"},
		1003: {PID: 300, Proc: ""},
	}
	var b bytes.Buffer
	if err := writeOwners(&b, in); err != nil {
		t.Fatal(err)
	}
	out, err := readOwners(bufio.NewReader(&b))
	if err != nil {
		t.Fatal(err)
	}
	if out[1001] != (sys.Owner{PID: 100, Proc: "sshd"}) || out[1003].PID != 300 || len(out) != 3 {
		t.Fatalf("got %+v", out)
	}
	if p := out[1002].Proc; strings.ContainsAny(p, " \x1b") {
		t.Fatalf("unsafe name crossed the pipe: %q", p)
	}
}

func TestReadOwnersRejects(t *testing.T) {
	for name, in := range map[string]string{
		"no terminator": "1 2 a\n",
		"bad inode":     "x 2 a\n.\n",
		"bad pid":       "1 -2 a\n.\n",
		"too many cols": "1 2 a b\n.\n",
		"long line":     "1 2 " + strings.Repeat("a", 200) + "\n.\n",
	} {
		if _, err := readOwners(bufio.NewReader(strings.NewReader(in))); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func FuzzReadOwners(f *testing.F) {
	f.Add("1 2 sshd\n.\n")
	f.Fuzz(func(t *testing.T, s string) {
		m, err := readOwners(bufio.NewReader(strings.NewReader(s)))
		if err != nil {
			return
		}
		for _, o := range m {
			if o.PID <= 0 || len(o.Proc) > maxComm {
				t.Fatalf("bad owner %+v", o)
			}
		}
	})
}

// The client against an in-process server over pipes, as the helper runs.
func TestClientServe(t *testing.T) {
	reqR, reqW := io.Pipe()
	respR, respW := io.Pipe()
	scan := func() map[uint64]sys.Owner { return map[uint64]sys.Owner{42: {PID: 7, Proc: "x"}} }
	go func() { _ = serve(reqR, respW, scan); respW.Close() }()
	c := &OwnerClient{w: reqW, r: bufio.NewReader(respR)}
	for range 3 {
		if got := c.Owners(); got[42] != (sys.Owner{PID: 7, Proc: "x"}) {
			t.Fatalf("got %+v", got)
		}
	}
	reqW.Close()
	// A dead helper degrades to the unprivileged local scan, not a hang.
	if got := c.Owners(); got == nil {
		t.Fatal("no fallback")
	}
	if !c.dead {
		t.Fatal("client did not notice the helper died")
	}
}
