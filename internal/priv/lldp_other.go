//go:build !linux

package priv

import (
	"errors"
	"io"

	"orb/internal/lldp"
)

// StartLLDPHelper is only implemented on Linux.
func StartLLDPHelper() (*lldp.Table, func(), error) { return nil, nil, errors.New("needs Linux") }

// RunLLDPHelper is only implemented on Linux.
func RunLLDPHelper(io.Reader, io.Writer) error { return errors.New("needs Linux") }
