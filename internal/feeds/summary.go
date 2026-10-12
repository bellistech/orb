package feeds

import (
	"html"
	"regexp"
	"strings"
)

// MinSummary is the shortest summary worth reading; shorter ones are
// usually link stubs ("Comments").
const MinSummary = 80

var (
	// Boilerplate feeds append; matched at the end of the cleaned text.
	trailers = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\s*The post .{0,200} appeared first on .{0,100}$`),
		regexp.MustCompile(`(?i)\s*(Continue reading|Read more|Read the full story|Read full article)\b.{0,40}$`),
		regexp.MustCompile(`(?i)\s*\(c\) SANS Internet Storm Center.*$`),
		regexp.MustCompile(`\s*\[(\.\.\.|…|&#8230;)\]\s*$`),
	}
	leaders = []*regexp.Regexp{
		regexp.MustCompile(`^(?i)View CSAF Summary\s*`),
	}
	// Elements whose content is never prose.
	dropElems = regexp.MustCompile(`(?is)<(script|style)\b.*?</(script|style)\s*>`)
)

// Summary turns a feed item's HTML description into plain text of at most
// max bytes, cut at a sentence end when it has to be cut.
func Summary(desc string, max int) string {
	if max <= 0 {
		return ""
	}
	// Feeds often escape their HTML once more; unescape until stable.
	for i := 0; i < 3 && strings.Contains(desc, "&"); i++ {
		u := html.UnescapeString(desc)
		if u == desc {
			break
		}
		desc = u
	}
	desc = dropElems.ReplaceAllString(desc, " ")
	desc = stripTags(desc)
	s := Clean(desc, 4*max+64)
	for _, re := range leaders {
		s = re.ReplaceAllString(s, "")
	}
	for _, re := range trailers {
		s = re.ReplaceAllString(s, "")
	}
	s = strings.TrimSpace(s)
	if s == "Comments" {
		return ""
	}
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	cut := s[:max]
	if i := strings.LastIndex(cut, ". "); i >= max/2 {
		return cut[:i+1]
	}
	if i := strings.LastIndexByte(cut, ' '); i > 0 {
		cut = cut[:i]
	}
	if len(cut)+3 > max {
		cut = cut[:max-3]
	}
	return cut + "..."
}

// stripTags removes anything that looks like an HTML tag: '<' followed by
// a letter, '/' or '!' up to the next '>'. A bare '<' (as in "a < b") stays.
func stripTags(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '<' && i+1 < len(s) && (isLetter(s[i+1]) || s[i+1] == '/' || s[i+1] == '!') {
			if j := strings.IndexByte(s[i:], '>'); j > 0 {
				b.WriteByte(' ')
				i += j
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
