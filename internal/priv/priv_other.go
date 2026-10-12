//go:build !linux

package priv

import (
	"errors"
	"io"
)

// HelperFlag is the hidden helper argument (unused off Linux).
const HelperFlag = "-sock-helper"

// Drop is only implemented on Linux.
func Drop(Target, ...Cap) error { return errors.New("privilege drop needs Linux") }

// StartHelper is only implemented on Linux.
func StartHelper() (*OwnerClient, error) { return nil, errors.New("needs Linux") }

// RunHelper is only implemented on Linux.
func RunHelper(io.Reader, io.Writer) error { return errors.New("needs Linux") }
