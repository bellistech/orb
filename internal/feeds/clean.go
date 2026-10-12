// Package feeds fetches the news and weather JSON the dashboard shows. All
// feed content is treated as hostile: bodies are size-capped, every string is
// reduced to printable ASCII, and numbers are range-checked.
package feeds

import (
	"html"
	"strings"
	"unicode"
)

// translit maps common typographic and Latin-1 characters to ASCII so
// headlines survive the reduction to printable ASCII.
var translit = map[rune]string{
	'‘': "'", '’': "'", '‚': "'", '′': "'", '“': `"`, '”': `"`, '„': `"`, '″': `"`,
	'–': "-", '—': "-", '‐': "-", '‑': "-", '−': "-", '…': "...", '•': "*", '·': "*",
	' ': " ", ' ': " ", ' ': " ", '×': "x", '©': "(c)", '®': "(R)", '™': "(TM)",
	'€': "EUR", '£': "GBP", '°': "deg",
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'æ': "ae", 'ç': "c",
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e", 'ì': "i", 'í': "i", 'î': "i", 'ï': "i",
	'ñ': "n", 'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o", 'ù': "u",
	'ú': "u", 'û': "u", 'ü': "u", 'ý': "y", 'ÿ': "y", 'ß': "ss",
	'À': "A", 'Á': "A", 'Â': "A", 'Ã': "A", 'Ä': "A", 'Å': "A", 'Æ': "AE", 'Ç': "C",
	'È': "E", 'É': "E", 'Ê': "E", 'Ë': "E", 'Ì': "I", 'Í': "I", 'Î': "I", 'Ï': "I",
	'Ñ': "N", 'Ò': "O", 'Ó': "O", 'Ô': "O", 'Õ': "O", 'Ö': "O", 'Ø': "O", 'Ù': "U",
	'Ú': "U", 'Û': "U", 'Ü': "U", 'Ý': "Y",
}

// Clean unescapes HTML entities, transliterates what it can, drops control
// characters (so nothing can smuggle terminal escapes), replaces anything
// else non-ASCII with '?', collapses whitespace, and cuts to max runes.
func Clean(s string, max int) string {
	if max <= 0 {
		return ""
	}
	s = html.UnescapeString(s)
	var b strings.Builder
	space := false
	for _, r := range s {
		var out string
		switch {
		case unicode.IsSpace(r):
			space = b.Len() > 0
			continue
		case r >= 0x20 && r < 0x7f:
			out = string(r)
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Mn, r):
			continue // control, format and combining marks
		case unicode.Is(unicode.So, r) || unicode.Is(unicode.Sk, r) || unicode.Is(unicode.Cs, r) || unicode.Is(unicode.Co, r):
			continue // emoji and other pictographs carry no words
		default:
			if t, ok := translit[r]; ok {
				out = t
			} else {
				out = "?"
			}
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteString(out)
		if b.Len() >= max {
			break
		}
	}
	out := b.String()
	if len(out) > max {
		out = out[:max] // ASCII only, so bytes are runes
	}
	return strings.TrimSpace(out)
}
