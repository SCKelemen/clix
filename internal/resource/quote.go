package resource

import (
	"strings"
)

// Expr returns k as an expression writes it, which Lexer.Key reads back
// as k: names joined with dots, [N] indices and [] slices, and "." for
// the empty key (the resource itself). A name that the lexer would read
// differently unquoted is in double quotes (see QuoteToken), and so is a
// name that starts with "-", which a filter term would read as NOT.
//
// It differs from String, gcloud's GetKeyName, only in the quoting.
func (k Key) Expr() string {
	if len(k) == 0 {
		return "."
	}
	var b strings.Builder
	for i, p := range k {
		switch {
		case p.Slice:
			b.WriteString("[]")
		case p.Index != nil:
			b.WriteString("[" + p.Index.String() + "]")
		default:
			if i > 0 {
				b.WriteByte('.')
			}
			if strings.HasPrefix(p.Name, "-") {
				b.WriteString(quote(p.Name))
			} else {
				b.WriteString(QuoteToken(p.Name, reservedOperatorChars))
			}
		}
	}
	return b.String()
}

// QuoteToken returns s as a token that Lexer.Token(terminators, ...)
// reads back as s: unchanged when it is not empty and has no whitespace,
// quote, backslash or terminator character, and otherwise in double
// quotes, with a backslash before each backslash and double quote.
func QuoteToken(s, terminators string) string {
	if s == "" || strings.ContainsFunc(s, func(r rune) bool {
		return isSpace(r) || r == escapeChar || strings.ContainsRune(quoteChars, r) || strings.ContainsRune(terminators, r)
	}) {
		return quote(s)
	}
	return s
}

// quote writes s in double quotes for Lexer.Token: inside them, a
// backslash before a backslash or a double quote makes it literal.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		if r == escapeChar || r == '"' {
			b.WriteRune(escapeChar)
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}
