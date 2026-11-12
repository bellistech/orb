//go:build !linux

package input

import "os"

// Open is only implemented on Linux.
func Open(string) []*os.File { return nil }

// Start is only implemented on Linux.
func Start(string) (<-chan struct{}, func()) { return nil, func() {} }
