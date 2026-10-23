package config

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, body string, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(p, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad(t *testing.T) {
	p := write(t, `# comment
theme = chrono
scale=5
fps = 30
interval = 2s
page = news
intro = false
attract = true
news_url = https://example.com/n.json
weather_url = https://example.com/w.json   # trailing comment
`, 0o600)
	c, warn, err := Load(p)
	if err != nil || warn != "" {
		t.Fatalf("err=%v warn=%q", err, warn)
	}
	want := Config{Theme: "chrono", Scale: 5, FPS: 30, HasFPS: true, Interval: 2 * time.Second, Page: "news",
		NoIntro: true, Attract: true, NewsURL: "https://example.com/n.json", WeatherURL: "https://example.com/w.json"}
	if c != want {
		t.Fatalf("got %+v\nwant %+v", c, want)
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	c, _, err := Load(filepath.Join(t.TempDir(), "nope"))
	if err != nil || c != (Config{}) {
		t.Fatalf("got %+v, %v", c, err)
	}
}

func TestLoadRejects(t *testing.T) {
	for name, body := range map[string]string{
		"unknown key":   "colour = red",
		"no equals":     "theme chrono",
		"bad scale":     "scale = 99",
		"bad fps":       "fps = -1",
		"bad interval":  "interval = soon",
		"tiny interval": "interval = 1ms",
		"bad page":      "page = sports",
		"bad intro":     "intro = yes",
		"bad attract":   "attract = 1",
		"http news":     "news_url = http://example.com/n.json",
		"duplicate":     "fps = 1\nfps = 2",
		"huge":          strings.Repeat("# x\n", 20000),
	} {
		if _, _, err := Load(write(t, body, 0o600)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestLoadWarnsWhenReadableByOthers(t *testing.T) {
	_, warn, err := Load(write(t, "fps = 10\n", 0o644))
	if err != nil || !strings.Contains(warn, "chmod 600") {
		t.Fatalf("warn=%q err=%v", warn, err)
	}
}

func TestFPSZeroIsSet(t *testing.T) {
	c, _, err := Load(write(t, "fps = 0\n", 0o600))
	if err != nil || !c.HasFPS || c.FPS != 0 {
		t.Fatalf("got %+v, %v", c, err)
	}
}

func TestBoolKeys(t *testing.T) {
	c, _, err := Load(write(t, "intro = true\nattract = false\n", 0o600))
	if err != nil || c != (Config{}) {
		t.Fatalf("defaults spelled out: got %+v, %v", c, err)
	}
}

func TestDefaultPathUnderSudo(t *testing.T) {
	lookup := func(name string) (*user.User, error) {
		if name == "alice" {
			return &user.User{HomeDir: "/home/alice"}, nil
		}
		return nil, errors.New("unknown user")
	}
	cfg := func() (string, error) { return "/root/.config", nil }
	for _, tc := range []struct {
		euid int
		sudo string
		want string
	}{
		{0, "alice", "/home/alice/.config/orb/config"}, // sudo orb
		{0, "", "/root/.config/orb/config"},            // root login
		{0, "root", "/root/.config/orb/config"},
		{0, "nobody-here", "/root/.config/orb/config"}, // unknown user: own config
		{1000, "alice", "/root/.config/orb/config"},    // not root: SUDO_USER ignored
	} {
		if got := defaultPath(tc.euid, tc.sudo, lookup, cfg); got != tc.want {
			t.Errorf("euid=%d SUDO_USER=%q: %s, want %s", tc.euid, tc.sudo, got, tc.want)
		}
	}
}
