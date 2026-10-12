package priv

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"orb/internal/sys"
)

// The socket-owner helper protocol, over the helper's stdin and stdout:
// the client writes "O\n"; the helper answers one "inode pid comm" line per
// socket and a final ".". comm never contains spaces or control bytes.

const (
	maxComm    = 64
	maxLine    = 128
	maxOwners  = sys.MaxFDScan
	requestTag = "O\n"
)

func safeComm(s string) string {
	b := []byte(s)
	if len(b) > maxComm {
		b = b[:maxComm]
	}
	for i, c := range b {
		if c <= ' ' || c >= 0x7f {
			b[i] = '_'
		}
	}
	return string(b)
}

func writeOwners(w io.Writer, m map[uint64]sys.Owner) error {
	bw := bufio.NewWriter(w)
	for inode, o := range m {
		comm := safeComm(o.Proc)
		if comm == "" {
			comm = "-"
		}
		fmt.Fprintf(bw, "%d %d %s\n", inode, o.PID, comm)
	}
	bw.WriteString(".\n")
	return bw.Flush()
}

// readOwners parses one reply strictly; the main process trusts nothing
// the helper sends.
func readOwners(r *bufio.Reader) (map[uint64]sys.Owner, error) {
	out := map[uint64]sys.Owner{}
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		if len(line) > maxLine {
			return nil, errors.New("helper: line too long")
		}
		line = strings.TrimSuffix(line, "\n")
		if line == "." {
			return out, nil
		}
		f := strings.Split(line, " ")
		if len(f) != 3 || len(out) >= maxOwners {
			return nil, errors.New("helper: malformed reply")
		}
		inode, err1 := strconv.ParseUint(f[0], 10, 64)
		pid, err2 := strconv.ParseUint(f[1], 10, 31)
		if err1 != nil || err2 != nil || pid == 0 || len(f[2]) > maxComm {
			return nil, errors.New("helper: malformed reply")
		}
		comm := f[2]
		if comm == "-" {
			comm = ""
		}
		out[inode] = sys.Owner{PID: int(pid), Proc: comm}
	}
}

// serve answers requests until r ends.
func serve(r io.Reader, w io.Writer, scan func() map[uint64]sys.Owner) error {
	br := bufio.NewReader(r)
	for {
		req, err := br.ReadString('\n')
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if req != requestTag {
			return errors.New("helper: unknown request")
		}
		if err := writeOwners(w, scan()); err != nil {
			return err
		}
	}
}

// OwnerClient asks the privileged helper for socket owners. If the helper
// dies it falls back to the local, unprivileged scan.
type OwnerClient struct {
	mu   sync.Mutex
	w    io.WriteCloser
	r    *bufio.Reader
	dead bool
	stop func()
}

// Owners implements sys.OwnerFunc.
func (c *OwnerClient) Owners() map[uint64]sys.Owner {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.dead {
		if _, err := io.WriteString(c.w, requestTag); err == nil {
			if m, err := readOwners(c.r); err == nil {
				return m
			}
		}
		c.dead = true
		if c.stop != nil {
			c.stop()
		}
	}
	return sys.LocalOwners()
}

// Close ends the helper.
func (c *OwnerClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.w.Close()
	if c.stop != nil {
		c.stop()
	}
	c.dead = true
}
