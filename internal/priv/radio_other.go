//go:build !linux

package priv

import (
	"errors"
	"io"
	"time"
)

// StartRadioHelper is only implemented on Linux.
func StartRadioHelper(time.Duration) (*RadioScans, func(), error) {
	return nil, nil, errors.New("needs Linux")
}

// RunRadioHelper is only implemented on Linux.
func RunRadioHelper([]string, io.Reader, io.Writer) error { return errors.New("needs Linux") }
