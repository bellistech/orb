package feeds

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// News is the parsed headline feed, grouped by category.
type News struct {
	Generated  time.Time
	Categories []Category
}

// Category is one group of headlines, newest first.
type Category struct {
	Name  string
	Items []Item
}

// Item is one headline.
type Item struct {
	Title     string
	Feed      string
	Published time.Time // zero when the feed gave none
	Summary   string    // plain text; "" when the feed gave nothing readable
}

const (
	maxCategories   = 32
	maxFeeds        = 64
	maxItemsPerFeed = 200
	maxItemsShown   = 60
	maxTitle        = 160
	maxSummary      = 1200
)

type rawNews struct {
	GeneratedAt string `json:"generated_at"`
	Categories  map[string]struct {
		Feeds []struct {
			FeedName string `json:"feed_name"`
			Items    []struct {
				Title       string `json:"title"`
				Published   string `json:"published"`
				Description string `json:"description"`
			} `json:"items"`
		} `json:"feeds"`
	} `json:"categories"`
}

// prettyNames for the categories the feed is known to carry.
var prettyNames = map[string]string{
	"networking":      "Networking",
	"sre_devops":      "SRE / DevOps",
	"linux_systems":   "Linux",
	"news_community":  "Community",
	"security":        "Security",
	"other_interests": "Other",
}

// categoryOrder keeps the known categories in a stable, sensible order.
var categoryOrder = []string{"security", "networking", "sre_devops", "linux_systems", "news_community", "other_interests"}

// ParseNews decodes the news feed, keeping the newest items per category.
func ParseNews(b []byte) (*News, error) {
	var r rawNews
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("news: %w", err)
	}
	if len(r.Categories) == 0 {
		return nil, fmt.Errorf("news: no categories")
	}
	n := &News{}
	n.Generated, _ = time.Parse(time.RFC3339Nano, r.GeneratedAt)

	keys := make([]string, 0, len(r.Categories))
	for k := range r.Categories {
		keys = append(keys, k)
	}
	rank := func(k string) int {
		for i, o := range categoryOrder {
			if o == k {
				return i
			}
		}
		return len(categoryOrder)
	}
	sort.Slice(keys, func(i, j int) bool {
		if rank(keys[i]) != rank(keys[j]) {
			return rank(keys[i]) < rank(keys[j])
		}
		return keys[i] < keys[j]
	})
	if len(keys) > maxCategories {
		keys = keys[:maxCategories]
	}
	for _, k := range keys {
		c := Category{Name: prettyName(k)}
		seen := map[string]bool{}
		for fi, f := range r.Categories[k].Feeds {
			if fi >= maxFeeds {
				break
			}
			feed := Clean(f.FeedName, 24)
			for ii, it := range f.Items {
				if ii >= maxItemsPerFeed {
					break
				}
				title := Clean(it.Title, maxTitle)
				if title == "" || seen[title] {
					continue
				}
				seen[title] = true
				pub, _ := time.Parse(time.RFC3339Nano, it.Published)
				sum := Summary(it.Description, maxSummary)
				if len(sum) < MinSummary {
					sum = ""
				}
				c.Items = append(c.Items, Item{Title: title, Feed: feed, Published: pub, Summary: sum})
			}
		}
		sort.SliceStable(c.Items, func(i, j int) bool {
			a, b := c.Items[i].Published, c.Items[j].Published
			if a.IsZero() != b.IsZero() {
				return b.IsZero()
			}
			return a.After(b)
		})
		if len(c.Items) > maxItemsShown {
			c.Items = c.Items[:maxItemsShown]
		}
		n.Categories = append(n.Categories, c)
	}
	return n, nil
}

func prettyName(k string) string {
	if p, ok := prettyNames[k]; ok {
		return p
	}
	name := Clean(strings.ReplaceAll(k, "_", " "), 20)
	if name == "" {
		return "?"
	}
	return strings.ToUpper(name[:1]) + name[1:]
}
