// Package priv drops root privileges to an unprivileged user, keeping only
// the capabilities orb needs, on every OS thread, and verifies the result.
package priv

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Cap is a Linux capability number.
type Cap uint

// Capabilities orb may keep.
const (
	CapSysPtrace    Cap = 19 // read other users' /proc/<pid>/fd for socket owners
	CapSysTTYConfig Cap = 26 // KDSETMODE/VT ioctls on a VT that is not our controlling tty
)

// Target is the user to become.
type Target struct{ UID, GID int }

// nobody is where a root login (no sudo) drops to.
var nobody = Target{65534, 65534}

// TargetFromEnv picks the invoking user from sudo's SUDO_UID/SUDO_GID, or
// nobody. It never returns root.
func TargetFromEnv(getenv func(string) string) Target {
	uid, err1 := strconv.ParseUint(getenv("SUDO_UID"), 10, 31)
	gid, err2 := strconv.ParseUint(getenv("SUDO_GID"), 10, 31)
	if err1 != nil || err2 != nil || uid == 0 || gid == 0 {
		return nobody
	}
	return Target{int(uid), int(gid)}
}

// Mask is the capability bitmask for caps.
func Mask(caps ...Cap) uint64 {
	var m uint64
	for _, c := range caps {
		m |= 1 << c
	}
	return m
}

// checkStatus verifies one thread's /proc status after a drop: every uid
// and gid is the target, no groups, no_new_privs, and every capability set
// is exactly mask (inheritable and ambient empty).
func checkStatus(status string, t Target, mask uint64) error {
	f := map[string]string{}
	for _, line := range strings.Split(status, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if ok {
			f[k] = strings.TrimSpace(v)
		}
	}
	ids := func(key string, want int) error {
		vals := strings.Fields(f[key])
		if len(vals) != 4 {
			return fmt.Errorf("%s: %q", key, f[key])
		}
		for _, v := range vals {
			if v != strconv.Itoa(want) {
				return fmt.Errorf("%s still %q", key, f[key])
			}
		}
		return nil
	}
	if err := ids("Uid", t.UID); err != nil {
		return err
	}
	if err := ids("Gid", t.GID); err != nil {
		return err
	}
	if g, ok := f["Groups"]; !ok || g != "" {
		return fmt.Errorf("supplementary groups kept: %q", g)
	}
	if f["NoNewPrivs"] != "1" {
		return errors.New("no_new_privs not set")
	}
	for key, want := range map[string]uint64{"CapEff": mask, "CapPrm": mask, "CapBnd": mask, "CapInh": 0, "CapAmb": 0} {
		got, err := strconv.ParseUint(f[key], 16, 64)
		if err != nil {
			return fmt.Errorf("%s: %q", key, f[key])
		}
		if got != want {
			return fmt.Errorf("%s = %016x, want %016x", key, got, want)
		}
	}
	return nil
}

func itoa(i int) string { return strconv.Itoa(i) }
