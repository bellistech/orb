package sys

import (
	"os"
	"strconv"
	"strings"
)

// readString returns the trimmed contents of path, or "" on any error.
func readString(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// readUint returns the integer in path, or ok=false.
func readUint(path string) (uint64, bool) {
	v, err := strconv.ParseUint(readString(path), 10, 64)
	return v, err == nil
}

// readInt returns the signed integer in path, or ok=false.
func readInt(path string) (int64, bool) {
	v, err := strconv.ParseInt(readString(path), 10, 64)
	return v, err == nil
}

func atou(s string) uint64 {
	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}
