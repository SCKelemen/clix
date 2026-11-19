package resource

import (
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// htmlTag is what NormalizeForSearch removes from HTML text.
var htmlTag = regexp.MustCompile(`<[^>]*>`)

// NormalizeForSearch is gcloud's resource_expr.NormalizeForSearch: the
// text of a value (StringizeForSearch) in lower case, with HTML tags
// removed when html is set, decomposed (Unicode NFKD) and with combining
// characters (canonical combining class other than 0) removed, so that
// "Café" and "cafe" read the same.
func NormalizeForSearch(v any, html bool) string {
	text := Lower(StringizeForSearch(v))
	if html {
		text = htmlTag.ReplaceAllString(text, "")
	}
	text = norm.NFKD.String(text)
	var b strings.Builder
	for _, r := range text {
		if norm.NFKD.PropertiesString(string(r)).CCC() != 0 {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// StringizeForSearch is gcloud's resource_expr._Stringize: "null" for
// None, a string as it is, and the repr of anything else.
func StringizeForSearch(v any) string {
	switch v := v.(type) {
	case nil:
		return "null"
	case string:
		return v
	}
	return Repr(v)
}
