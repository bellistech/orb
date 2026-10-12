package priv

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
)

func TestTargetFromEnv(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	for _, tc := range []struct {
		env  map[string]string
		want Target
	}{
		{map[string]string{"SUDO_UID": "1000", "SUDO_GID": "1000"}, Target{1000, 1000}},
		{map[string]string{}, Target{65534, 65534}},                                  // root login: nobody
		{map[string]string{"SUDO_UID": "0", "SUDO_GID": "0"}, Target{65534, 65534}},  // never stay root
		{map[string]string{"SUDO_UID": "-5", "SUDO_GID": "x"}, Target{65534, 65534}}, // garbage
		{map[string]string{"SUDO_UID": "1000"}, Target{65534, 65534}},                // half set
		{map[string]string{"SUDO_UID": "99999999999", "SUDO_GID": "1"}, Target{65534, 65534}},
	} {
		if got := TargetFromEnv(env(tc.env)); got != tc.want {
			t.Errorf("%v: got %+v, want %+v", tc.env, got, tc.want)
		}
	}
}

const statusOK = `Name:	orb
Uid:	1000	1000	1000	1000
Gid:	1000	1000	1000	1000
Groups:	
NoNewPrivs:	1
CapInh:	0000000000000000
CapPrm:	0000000000080000
CapEff:	0000000000080000
CapBnd:	0000000000080000
CapAmb:	0000000000000000
`

func TestCheckStatus(t *testing.T) {
	want := Target{1000, 1000}
	mask := Mask(CapSysPtrace)
	if err := checkStatus(statusOK, want, mask); err != nil {
		t.Fatalf("good status rejected: %v", err)
	}
	for name, mutate := range map[string]func(string) string{
		"still root": func(s string) string {
			return strings.Replace(s, "Uid:\t1000\t1000\t1000\t1000", "Uid:\t1000\t0\t1000\t1000", 1)
		},
		"extra cap": func(s string) string {
			return strings.Replace(s, "CapEff:\t0000000000080000", "CapEff:\t00000000000c0000", 1)
		},
		"bounding left": func(s string) string {
			return strings.Replace(s, "CapBnd:\t0000000000080000", "CapBnd:\t000001ffffffffff", 1)
		},
		"ambient": func(s string) string {
			return strings.Replace(s, "CapAmb:\t0000000000000000", "CapAmb:\t0000000000080000", 1)
		},
		"groups kept":  func(s string) string { return strings.Replace(s, "Groups:\t", "Groups:\t44 ", 1) },
		"no nnp":       func(s string) string { return strings.Replace(s, "NoNewPrivs:\t1", "NoNewPrivs:\t0", 1) },
		"missing line": func(s string) string { return strings.Replace(s, "CapPrm:\t0000000000080000\n", "", 1) },
	} {
		if err := checkStatus(mutate(statusOK), want, mask); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// The Drop tests do the real thing. They run only as (namespace) root with
// ORB_PRIV_TEST=1, one per process (a drop cannot be undone); make test-priv
// arranges that with unshare --map-auto --map-root-user and a static binary.
func needRoot(t *testing.T) *exec.Cmd {
	t.Helper()
	if os.Getenv("ORB_PRIV_TEST") != "1" || os.Geteuid() != 0 {
		t.Skip("run via make test-priv")
	}
	// A root-owned process to inspect after dropping.
	sleeper := exec.Command("sleep", "30")
	if err := sleeper.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sleeper.Process.Kill() })
	return sleeper
}

func socketLinks(pid int) error {
	dir := "/proc/" + itoa(pid) + "/fd"
	ents, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range ents {
		if _, err := os.Readlink(dir + "/" + e.Name()); err != nil {
			return err
		}
	}
	return nil
}

// The helper's capabilities are exactly enough to read another user's fds.
func TestDropHelperCaps(t *testing.T) {
	sleeper := needRoot(t)
	if err := Drop(nobody, CapDacReadSearch, CapSysPtrace); err != nil {
		t.Fatalf("Drop: %v", err)
	}
	if os.Geteuid() != nobody.UID || os.Getegid() != nobody.GID {
		t.Fatalf("uid %d gid %d", os.Geteuid(), os.Getegid())
	}
	if err := syscall.Setuid(0); err == nil {
		t.Fatal("could become root again")
	}
	if err := socketLinks(sleeper.Process.Pid); err != nil {
		t.Fatalf("helper caps cannot read another user's fds: %v", err)
	}
}

// The main process keeps nothing: other users' fds are out of reach.
func TestDropMainNoCaps(t *testing.T) {
	sleeper := needRoot(t)
	if err := Drop(Target{1000, 1000}); err != nil {
		t.Fatalf("Drop: %v", err)
	}
	if os.Geteuid() != 1000 {
		t.Fatalf("uid %d", os.Geteuid())
	}
	if err := socketLinks(sleeper.Process.Pid); err == nil {
		t.Fatal("main process can still read another user's fds")
	}
	if err := syscall.Setuid(0); err == nil {
		t.Fatal("could become root again")
	}
}

// Keeping CAP_SYS_TTY_CONFIG (for giving a taken console back) verifies.
func TestDropMainTTYConfig(t *testing.T) {
	needRoot(t)
	if err := Drop(Target{1000, 1000}, CapSysTTYConfig); err != nil {
		t.Fatalf("Drop: %v", err)
	}
}
