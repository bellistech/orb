//go:build unix

package config

import (
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

// openOwned opens path only if it is a regular file owned by uid. The type
// is checked before opening (opening a device can have side effects), the
// open refuses a symlink and does not block, and the opened file must be
// the one checked.
func openOwned(path string, uid int) (*os.File, fs.FileInfo, error) {
	pre, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if !pre.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("%s: not a regular file (symlinks are not followed as root)", path)
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, err
	}
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || !os.SameFile(pre, st) {
		f.Close()
		return nil, nil, fmt.Errorf("%s: changed while opening", path)
	}
	if sys, ok := st.Sys().(*syscall.Stat_t); !ok || int(sys.Uid) != uid {
		f.Close()
		return nil, nil, fmt.Errorf("%s: not owned by uid %d", path, uid)
	}
	return f, st, nil
}
