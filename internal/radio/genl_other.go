//go:build !linux

package radio

import (
	"errors"
	"time"
)

var errNotLinux = errors.New("Wi-Fi listening needs Linux")

// Interfaces is only available on Linux.
func Interfaces() ([]Iface, error) { return nil, errNotLinux }

// Heard is only available on Linux.
func Heard(int, string, time.Time) ([]AP, error) { return nil, errNotLinux }

// TriggerPassive is only available on Linux.
func TriggerPassive(int) error { return errNotLinux }
