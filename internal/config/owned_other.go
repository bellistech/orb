//go:build !unix

package config

import (
	"errors"
	"io/fs"
	"os"
)

func openOwned(string, int) (*os.File, fs.FileInfo, error) {
	return nil, nil, errors.New("reading a config as root is only supported on Unix")
}
