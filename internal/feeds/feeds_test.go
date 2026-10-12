package feeds

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClean(t *testing.T) {
	for in, want := range map[string]string{
		"Sandy Hook’s Artist":          "Sandy Hook's Artist",
		"“quoted” — dash – en…":        `"quoted" - dash - en...`,
		"Café naïve Å":                 "Cafe naive A",
		"a &amp; b &lt;tag&gt;":        "a & b <tag>",
		"evil\x1b[2Jclear\x07bell":     "evil[2Jclearbell",
		"tabs\tand\n\nnewlines   here": "tabs and newlines here",
		"日本語":                          "???",
		"  trim  ":                     "trim",
	} {
		if got := Clean(in, 100); got != want {
			t.Errorf("Clean(%q) = %q, want %q", in, got, want)
		}
	}
	if got := Clean(strings.Repeat("x", 500), 10); len(got) != 10 {
		t.Errorf("not truncated to 10: %q", got)
	}
}

func FuzzClean(f *testing.F) {
	f.Add("hello\x1b]0;pwn\x07", 40)
	f.Add("“x”", 3)
	f.Fuzz(func(t *testing.T, s string, n int) {
		n = n%300 + 1
		if n < 1 {
			n = 1
		}
		out := Clean(s, n)
		if len([]rune(out)) > n {
			t.Fatalf("len %d > %d", len([]rune(out)), n)
		}
		for _, r := range out {
			if r < 0x20 || r > 0x7e {
				t.Fatalf("non-printable %U in %q", r, out)
			}
		}
	})
}

const weatherJSON = `{"location":"Austin, Texas","updated":"2026-10-12T16:21:52.212186+00:00",
 "now":{"temp":80,"summary":"Mostly cloudy","icon":"☁️","high":91,"low":72,"precip_chance":10},
 "hourly":[{"time":"11 AM","temp":79,"icon":"☁️"},{"time":"12 PM","temp":83,"icon":"⛈️"}],
 "daily":[{"day":"Monday","high":89,"low":72,"summary":"Scattered thunderstorms","icon":"🌤️"}]}`

func TestParseWeather(t *testing.T) {
	w, err := ParseWeather([]byte(weatherJSON))
	if err != nil {
		t.Fatal(err)
	}
	if w.Location != "Austin, Texas" || w.Now.Temp != 80 || w.Now.Kind != Cloudy || w.Now.Precip != 10 {
		t.Fatalf("now = %+v", w.Now)
	}
	if len(w.Hourly) != 2 || w.Hourly[1].Kind != Storm || w.Hourly[0].Label != "11 AM" {
		t.Fatalf("hourly = %+v", w.Hourly)
	}
	// The emoji says partly cloudy but the summary says storms: summary wins.
	if len(w.Daily) != 1 || w.Daily[0].Kind != Storm || w.Daily[0].Name != "Monday" {
		t.Fatalf("daily = %+v", w.Daily)
	}
	if w.Updated.IsZero() {
		t.Fatal("updated not parsed")
	}
}

func TestParseWeatherRejects(t *testing.T) {
	for name, in := range map[string]string{
		"not json":     `<html>`,
		"no now":       `{"location":"x"}`,
		"absurd temp":  `{"now":{"temp":9000,"high":1,"low":1}}`,
		"bad hourly":   `{"now":{"temp":70,"high":80,"low":60},"hourly":[{"time":"1","temp":-500}]}`,
		"precip > 100": `{"now":{"temp":70,"high":80,"low":60,"precip_chance":250}}`,
	} {
		if _, err := ParseWeather([]byte(in)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

const newsJSON = `{"generated_at":"2026-10-12T17:01:03+00:00","categories":{
 "networking":{"feeds":[
   {"feed_name":"ipSpace.net","items":[
     {"title":"Older","published":"2026-10-10T00:00:00+00:00"},
     {"title":"Newest &amp; best","published":"2026-10-12T04:37:00+00:00"}]},
   {"feed_name":"Other","items":[
     {"title":"Newest &amp; best","published":"2026-10-12T04:37:00+00:00"},
     {"title":"","published":"2026-10-11T00:00:00+00:00"},
     {"title":"No date"}]}]},
 "sre_devops":{"feeds":[]}}}`

func TestParseNews(t *testing.T) {
	n, err := ParseNews([]byte(newsJSON))
	if err != nil {
		t.Fatal(err)
	}
	if len(n.Categories) != 2 {
		t.Fatalf("categories = %d", len(n.Categories))
	}
	c := n.Categories[0]
	if c.Name != "Networking" {
		t.Fatalf("name = %q", c.Name)
	}
	// Sorted newest first, duplicate titles and empty titles dropped,
	// undated items last.
	var titles []string
	for _, it := range c.Items {
		titles = append(titles, it.Title)
	}
	if got := strings.Join(titles, "|"); got != "Newest & best|Older|No date" {
		t.Fatalf("items = %q", got)
	}
	if c.Items[0].Feed != "ipSpace.net" {
		t.Fatalf("feed = %q", c.Items[0].Feed)
	}
	if n.Categories[1].Name != "SRE / DevOps" {
		t.Fatalf("name = %q", n.Categories[1].Name)
	}
}

func TestFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "orb/test" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/ok":
			w.Write([]byte(weatherJSON))
		case "/big":
			w.Write([]byte(strings.Repeat("x", MaxBody+10)))
		case "/slow":
			time.Sleep(300 * time.Millisecond)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := NewClient("orb/test", 100*time.Millisecond)
	ctx := context.Background()
	if b, err := c.Get(ctx, srv.URL+"/ok"); err != nil || len(b) == 0 {
		t.Fatalf("ok: %v", err)
	}
	if _, err := c.Get(ctx, srv.URL+"/big"); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("big: %v", err)
	}
	if _, err := c.Get(ctx, srv.URL+"/missing"); err == nil || Brief(err) != "HTTP 404" {
		t.Fatalf("404: %v / %q", err, Brief(err))
	}
	if _, err := c.Get(ctx, srv.URL+"/slow"); err == nil || Brief(err) != "timeout" {
		t.Fatalf("slow: %v / %q", err, Brief(err))
	}
}

func TestCheckURL(t *testing.T) {
	for _, ok := range []string{"https://bellis.tech/x.json", "http://127.0.0.1:8080/x", "http://[::1]/x", "http://localhost/x"} {
		if err := CheckURL(ok); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "ftp://x/y", "http://bellis.tech/x", "file:///etc/passwd", "https://", "https://user:pw@host/x"} {
		if err := CheckURL(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// Errors shown on screen must never contain the feed URL: the news URL is
// a private capability.
func TestBriefHidesURL(t *testing.T) {
	c := NewClient("orb/test", 50*time.Millisecond)
	_, err := c.Get(context.Background(), "http://127.0.0.1:1/secret_path_123.json")
	if err == nil {
		t.Fatal("expected error")
	}
	if b := Brief(err); strings.Contains(b, "secret") || strings.Contains(b, "127.0.0.1") {
		t.Fatalf("Brief leaks: %q", b)
	}
}

func TestStorePoll(t *testing.T) {
	calls := 0
	s := &Store{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		Poll(ctx, 10*time.Millisecond, func(context.Context) error {
			calls++
			if calls == 1 {
				return errors.New("boom")
			}
			s.SetWeather(&Weather{Location: "x"}, nil)
			cancel()
			return nil
		}, func(err error) { s.SetWeather(nil, err) })
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("poll did not retry after an error")
	}
	v := s.View()
	if v.Weather == nil || v.WeatherErr != "" || v.Version < 2 {
		t.Fatalf("view = %+v", v)
	}
}

func TestSummary(t *testing.T) {
	for in, want := range map[string]string{
		"&lt;p&gt;With the &lt;a href=&quot;/x&quot;&gt;growing number&lt;/a&gt; of SRv6.&lt;/p&gt;": "With the growing number of SRv6.",
		"<p>Body text here.</p><p>The post Foo appeared first on Bar.</p>":                           "Body text here.",
		"Some words. Continue reading →":                                                             "Some words.",
		"Agents arrested someone. Read more":                                                         "Agents arrested someone.",
		`<a href="https://news.ycombinator.com/item?id=1">Comments</a>`:                              "",
		"<script>alert(1)</script>Real text":                                                         "Real text",
		"a < b and c > d":                                                                            "a < b and c > d",
	} {
		if got := Summary(in, 400); got != want {
			t.Errorf("Summary(%q) = %q, want %q", in, got, want)
		}
	}
	long := strings.Repeat("Sentence one is here. ", 40)
	got := Summary(long, 100)
	if len(got) > 100 || !strings.HasSuffix(got, "here.") {
		t.Errorf("long summary not cut at a sentence: %q", got)
	}
}

func FuzzSummary(f *testing.F) {
	f.Add("<p>x</p>", 50)
	f.Add("&lt;b&gt;&amp;amp;", 10)
	f.Fuzz(func(t *testing.T, s string, n int) {
		n = n%2000 + 1
		if n < 1 {
			n = 1
		}
		out := Summary(s, n)
		if len(out) > n {
			t.Fatalf("len %d > %d", len(out), n)
		}
		for _, r := range out {
			if r < 0x20 || r > 0x7e {
				t.Fatalf("non-printable %U", r)
			}
		}
	})
}
