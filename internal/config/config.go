// Package config reads ~/.config/nocterm/config: "key = value" lines, '#'
// comments. It holds settings that should not live on the command line or in
// the repo, such as private feed URLs. Command-line flags override it.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"nocterm/internal/feeds"
)

// Config is every setting the file may hold. Zero values mean unset.
type Config struct {
	Theme      string
	Scale      int
	FPS        int
	HasFPS     bool // fps = 0 (no animation) is a real setting
	Interval   time.Duration
	Page       string
	NewsURL    string
	WeatherURL string
}

const maxSize = 64 << 10

// DefaultPath is $XDG_CONFIG_HOME/nocterm/config or ~/.config/nocterm/config.
func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "nocterm", "config")
}

// Load parses the file at path. A missing file is an empty config. warn is
// non-empty when the file is readable by others (it may hold private URLs).
func Load(path string) (c Config, warn string, err error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, "", nil
	}
	if err != nil {
		return Config{}, "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return Config{}, "", err
	}
	if st.Size() > maxSize {
		return Config{}, "", fmt.Errorf("%s: larger than %d bytes", path, maxSize)
	}
	if st.Mode().Perm()&0o077 != 0 {
		warn = fmt.Sprintf("%s is readable by others; run: chmod 600 %s", path, path)
	}

	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return Config{}, "", fmt.Errorf("%s:%d: want key = value", path, n)
		}
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		if seen[key] {
			return Config{}, "", fmt.Errorf("%s:%d: %s set twice", path, n, key)
		}
		seen[key] = true
		if err := c.set(key, val); err != nil {
			return Config{}, "", fmt.Errorf("%s:%d: %w", path, n, err)
		}
	}
	if err := sc.Err(); err != nil {
		return Config{}, "", err
	}
	return c, warn, nil
}

func (c *Config) set(key, val string) error {
	var err error
	switch key {
	case "theme":
		c.Theme = val
	case "scale":
		c.Scale, err = intIn(val, 1, 8)
	case "fps":
		c.FPS, err = intIn(val, 0, 60)
		c.HasFPS = err == nil
	case "interval":
		c.Interval, err = time.ParseDuration(val)
		if err == nil && (c.Interval < 100*time.Millisecond || c.Interval > time.Hour) {
			err = errors.New("interval must be 100ms..1h")
		}
	case "page":
		if val != "dash" && val != "news" {
			return errors.New("page must be dash or news")
		}
		c.Page = val
	case "news_url":
		c.NewsURL, err = val, feeds.CheckURL(val)
	case "weather_url":
		c.WeatherURL, err = val, feeds.CheckURL(val)
	default:
		return fmt.Errorf("unknown key %q", key)
	}
	return err
}

func intIn(s string, lo, hi int) (int, error) {
	v, err := strconv.Atoi(s)
	if err != nil || v < lo || v > hi {
		return 0, fmt.Errorf("want a number %d..%d", lo, hi)
	}
	return v, nil
}
