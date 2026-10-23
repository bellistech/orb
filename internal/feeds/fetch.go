package feeds

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// MaxBody caps a feed response. The news feed is about 1 MB.
const MaxBody = 8 << 20

// ErrTooLarge means a response exceeded MaxBody.
var ErrTooLarge = errors.New("response too large")

// Client fetches feeds with a fixed User-Agent and timeout.
type Client struct {
	http *http.Client
	ua   string
}

// NewClient returns a client. Redirects are limited, never leave https
// for http, and carry no Referer: the client would otherwise hand the
// private feed URL to wherever the redirect points.
func NewClient(userAgent string, timeout time.Duration) *Client {
	return &Client{
		ua: userAgent,
		http: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				req.Header.Del("Referer")
				if len(via) >= 3 {
					return errors.New("too many redirects")
				}
				if via[0].URL.Scheme == "https" && req.URL.Scheme != "https" {
					return errors.New("redirect leaves https")
				}
				return CheckURL(req.URL.String())
			},
		},
	}
}

// CheckURL accepts https URLs, and plain http only to the local machine.
func CheckURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Hostname() == "" {
		return errors.New("feed URL must be an absolute http(s) URL")
	}
	if u.User != nil {
		return errors.New("feed URL must not carry credentials")
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		h := u.Hostname()
		if h == "localhost" {
			return nil
		}
		if ip := net.ParseIP(h); ip != nil && ip.IsLoopback() {
			return nil
		}
		return errors.New("feed URL must use https (http only for localhost)")
	}
	return errors.New("feed URL must use https")
}

type statusError int

func (e statusError) Error() string { return fmt.Sprintf("HTTP %d", int(e)) }

// Get fetches url and returns at most MaxBody bytes.
func (c *Client) Get(ctx context.Context, rawURL string) ([]byte, error) {
	if err := CheckURL(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, statusError(resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, MaxBody+1))
	if err != nil {
		return nil, err
	}
	if len(b) > MaxBody {
		return nil, ErrTooLarge
	}
	return b, nil
}

// Brief turns an error into a few words safe to show on screen. It never
// includes the URL: the news URL is a private capability.
func Brief(err error) string {
	var se statusError
	var ne net.Error
	var de *net.DNSError
	switch {
	case err == nil:
		return ""
	case errors.As(err, &se):
		return se.Error()
	case errors.Is(err, ErrTooLarge):
		return "response too large"
	case errors.Is(err, context.DeadlineExceeded) || errors.As(err, &ne) && ne.Timeout():
		return "timeout"
	case errors.As(err, &de):
		return "DNS lookup failed"
	case errors.Is(err, errBadData):
		return "bad data"
	}
	var oe *net.OpError
	if errors.As(err, &oe) {
		return "connection failed"
	}
	return "fetch failed"
}

var errBadData = errors.New("bad data")

// Poll calls fetch now and then every interval until ctx ends. After a
// failure it retries sooner (30s, doubling, never past interval) and
// reports the error through onErr.
func Poll(ctx context.Context, interval time.Duration, fetch func(context.Context) error, onErr func(error)) {
	retry := min(30*time.Second, interval)
	for {
		wait := interval
		if err := fetch(ctx); err != nil {
			if onErr != nil {
				onErr(err)
			}
			wait = retry
			retry = min(retry*2, interval)
		} else {
			retry = min(30*time.Second, interval)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// Store holds the latest feed data for the UI. Safe for concurrent use.
type Store struct {
	mu   sync.RWMutex
	view View
}

// View is a consistent snapshot of the store.
type View struct {
	News       *News
	Weather    *Weather
	NewsErr    string // brief, screen-safe
	WeatherErr string
	NewsAt     time.Time // last successful fetch
	WeatherAt  time.Time
	NewsOn     bool // a URL is configured
	WeatherOn  bool
	Version    uint64 // bumps on every change
}

// View returns the current snapshot. The pointed-to data is never mutated
// after being stored.
func (s *Store) View() View {
	if s == nil {
		return View{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.view
}

// SetNews stores news, or an error that keeps the previous news.
func (s *Store) SetNews(n *News, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.view.Version++
	if err != nil {
		s.view.NewsErr = Brief(err)
		return
	}
	s.view.News, s.view.NewsErr, s.view.NewsAt = n, "", time.Now()
}

// SetWeather stores weather, or an error that keeps the previous weather.
func (s *Store) SetWeather(w *Weather, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.view.Version++
	if err != nil {
		s.view.WeatherErr = Brief(err)
		return
	}
	s.view.Weather, s.view.WeatherErr, s.view.WeatherAt = w, "", time.Now()
}

// Start polls the configured URLs into s until ctx ends. Empty URLs are
// skipped.
func (s *Store) Start(ctx context.Context, c *Client, newsURL, weatherURL string) {
	s.mu.Lock()
	s.view.NewsOn, s.view.WeatherOn = newsURL != "", weatherURL != ""
	s.mu.Unlock()
	if newsURL != "" {
		go Poll(ctx, 15*time.Minute, func(ctx context.Context) error {
			b, err := c.Get(ctx, newsURL)
			if err != nil {
				return err
			}
			n, err := ParseNews(b)
			if err != nil {
				return fmt.Errorf("%w: %v", errBadData, err)
			}
			s.SetNews(n, nil)
			return nil
		}, func(err error) { s.SetNews(nil, err) })
	}
	if weatherURL != "" {
		go Poll(ctx, 10*time.Minute, func(ctx context.Context) error {
			b, err := c.Get(ctx, weatherURL)
			if err != nil {
				return err
			}
			w, err := ParseWeather(b)
			if err != nil {
				return fmt.Errorf("%w: %v", errBadData, err)
			}
			s.SetWeather(w, nil)
			return nil
		}, func(err error) { s.SetWeather(nil, err) })
	}
}
