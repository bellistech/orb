package priv

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"

	"orb/internal/lldp"
)

// The LLDP helper protocol, helper -> main over the helper's stdout: one
// "R" line once the helper has dropped, then one JSON object per neighbor
// heard: {"if": "<interface>", "n": <lldp.Neighbor>}. The main process
// re-validates every line; a malformed one ends the helper.

// LLDPHelperFlag is the hidden argument that runs the LLDP helper.
const LLDPHelperFlag = "-lldp-helper"

const (
	lldpReady   = "R"
	maxLLDPLine = 4096
)

type lldpMsg struct {
	If string        `json:"if"`
	N  lldp.Neighbor `json:"n"`
}

// validIf accepts what an interface name can be (1-15 bytes of letters,
// digits and ._-@:), or "#<index>" for one the helper could not name.
func validIf(s string) bool {
	if s == "" || len(s) > 15 {
		return false
	}
	for _, c := range []byte(s) {
		ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-' || c == '@' || c == ':' || c == '#'
		if !ok {
			return false
		}
	}
	return true
}

func writeLLDP(w io.Writer, iface string, n lldp.Neighbor) error {
	b, err := json.Marshal(lldpMsg{If: iface, N: n})
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

// readLLDP parses one line strictly.
func readLLDP(line []byte) (string, lldp.Neighbor, error) {
	var m lldpMsg
	if len(line) > maxLLDPLine || json.Unmarshal(line, &m) != nil || !validIf(m.If) || m.N.Check() != nil {
		return "", lldp.Neighbor{}, errors.New("lldp helper: malformed line")
	}
	return m.If, m.N, nil
}

// pumpLLDP feeds t from the helper's output until it ends or misbehaves.
func pumpLLDP(r *bufio.Reader, t *lldp.Table) error {
	for {
		line, err := r.ReadSlice('\n')
		if err != nil {
			return err // includes bufio.ErrBufferFull: a line too long
		}
		iface, n, err := readLLDP(line[:len(line)-1])
		if err != nil {
			return err
		}
		t.Heard(iface, n)
	}
}
