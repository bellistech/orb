//go:build !linux

package sys

func readL2L3(func(int) string) ([]Neighbor, []Route, []FDBEntry) { return nil, nil, nil }
