//go:build !linux

package priv

import (
	"errors"
	"io"
)

// StartInputHelper is only implemented on Linux.
func StartInputHelper() (<-chan struct{}, func(), error) {
	return nil, nil, errors.New("needs Linux")
}

// RunInputHelper is only implemented on Linux.
func RunInputHelper(io.Reader, io.Writer) error { return errors.New("needs Linux") }
