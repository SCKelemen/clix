package jq

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// compileRegex compiles a test() pattern with Go's regexp (RE2) after
// checking that it uses only constructs that mean the same in RE2 and in
// jq's regex engine, Oniguruma (Perl syntax, single-line mode). Anything
// else is an *UnsupportedError, so a pattern never matches differently
// than it would in jq.
//
// Accepted: literal characters; "."; "^" (start of text); "$" at the end
// of the pattern or of an alternative (it matches at the end of the text
// or before a final newline, as in jq); groups (...), (?:...) and
// (?<name>...); alternation; the quantifiers *, +, ?, {n}, {n,} and {n,m},
// each optionally lazy (?); bracket classes with literal characters,
// ranges and negation; the escapes \t \n \r \f, \xHH, \x{H...}, \p{...}
// and \P{...} (Unicode general categories other than C, and scripts), \A
// and \z, and a backslash before an ASCII punctuation character.
func compileRegex(pattern string) (*regexp.Regexp, error) {
	rp := &reParser{src: pattern}
	tree, err := rp.alternation(0)
	if err != nil {
		return nil, err
	}
	if rp.pos < len(rp.src) {
		return nil, rp.errorf("unmatched )")
	}
	if err := checkDollar(tree, true); err != nil {
		return nil, err
	}
	if err := checkRepeatedAnchors(tree, false); err != nil {
		return nil, err
	}
	var b strings.Builder
	tree.emit(&b)
	re, err := regexp.Compile(b.String())
	if err != nil {
		// The pattern passed the checks above, so jq would accept it
		// (for example, a repeat count over RE2's limit of 1000).
		return nil, &UnsupportedError{Offset: -1, Construct: fmt.Sprintf("regex %q", pattern), Hint: err.Error()}
	}
	return re, nil
}

type reKind int

const (
	reAlt reKind = iota
	reConcat
	reGroup
	reRepeat
	reText  // Go syntax for one character, a class or an escape
	reBegin // ^ or \A
	reEnd   // $
	reZ     // \z
)

type reNode struct {
	kind    reKind
	subs    []*reNode
	text    string // reText: Go syntax; reRepeat: the quantifier
	capture bool   // reGroup: (...) or (?<name>...), not (?:...)
}

func (n *reNode) emit(b *strings.Builder) {
	switch n.kind {
	case reAlt:
		for i, s := range n.subs {
			if i > 0 {
				b.WriteByte('|')
			}
			s.emit(b)
		}
	case reConcat:
		for _, s := range n.subs {
			s.emit(b)
		}
	case reGroup:
		b.WriteString("(?:")
		n.subs[0].emit(b)
		b.WriteByte(')')
	case reRepeat:
		n.subs[0].emit(b)
		b.WriteString(n.text)
	case reText:
		b.WriteString(n.text)
	case reBegin:
		b.WriteString(`\A`)
	case reZ:
		b.WriteString(`\z`)
	case reEnd:
		// Oniguruma's "$" in single-line mode is \Z: the end of the text,
		// or just before a newline that ends it.
		b.WriteString(`(?:\n?\z)`)
	}
}

// checkDollar rejects a "$" that is not at the end of the pattern or of an
// alternative, where rewriting it as \n?\z would not match the same texts.
func checkDollar(n *reNode, tail bool) error {
	switch n.kind {
	case reAlt:
		for _, s := range n.subs {
			if err := checkDollar(s, tail); err != nil {
				return err
			}
		}
	case reConcat:
		for i, s := range n.subs {
			if err := checkDollar(s, tail && i == len(n.subs)-1); err != nil {
				return err
			}
		}
	case reGroup:
		return checkDollar(n.subs[0], tail)
	case reRepeat:
		return checkDollar(n.subs[0], false)
	case reEnd:
		if !tail {
			return &UnsupportedError{Offset: -1, Construct: "regex $ before the end of the pattern", Hint: `"$" must end the pattern or one of its alternatives`}
		}
	}
	return nil
}

// checkRepeatedAnchors rejects an anchor inside a quantified group, as in
// (^a)*: Oniguruma ends a loop at an iteration that matched nothing, RE2
// does not, and with an anchor in the loop that changes what matches.
func checkRepeatedAnchors(n *reNode, repeated bool) error {
	switch n.kind {
	case reBegin, reZ:
		if repeated {
			return &UnsupportedError{Offset: -1, Construct: `regex anchors (^, \A, \z) inside a quantified group`}
		}
	case reRepeat:
		return checkRepeatedAnchors(n.subs[0], true)
	default:
		for _, s := range n.subs {
			if err := checkRepeatedAnchors(s, repeated); err != nil {
				return err
			}
		}
	}
	return nil
}

type reParser struct {
	src string
	pos int
}

func (p *reParser) errorf(format string, args ...any) error {
	return fmt.Errorf("invalid regex %q: %s", p.src, fmt.Sprintf(format, args...))
}

func unsupportedRegex(construct, hint string) error {
	return &UnsupportedError{Offset: -1, Construct: "regex " + construct, Hint: hint}
}

func (p *reParser) peek() byte {
	if p.pos < len(p.src) {
		return p.src[p.pos]
	}
	return 0
}

func (p *reParser) alternation(depth int) (*reNode, error) {
	if depth > maxNesting {
		return nil, p.errorf("groups nest more than %d levels deep", maxNesting)
	}
	alt := &reNode{kind: reAlt}
	for {
		c, err := p.concat(depth)
		if err != nil {
			return nil, err
		}
		alt.subs = append(alt.subs, c)
		if p.peek() != '|' {
			break
		}
		p.pos++
	}
	if len(alt.subs) == 1 {
		return alt.subs[0], nil
	}
	return alt, nil
}

func (p *reParser) concat(depth int) (*reNode, error) {
	cat := &reNode{kind: reConcat}
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == '|' {
			break
		}
		if c == ')' {
			if depth == 0 {
				return nil, p.errorf("unmatched )")
			}
			break
		}
		atom, err := p.atom(depth)
		if err != nil {
			return nil, err
		}
		atom, err = p.quantifier(atom)
		if err != nil {
			return nil, err
		}
		cat.subs = append(cat.subs, atom)
	}
	return cat, nil
}

func isQuantifierStart(c byte) bool { return c == '*' || c == '+' || c == '?' || c == '{' }

// quantifier parses the quantifiers after atom.
func (p *reParser) quantifier(atom *reNode) (*reNode, error) {
	if !isQuantifierStart(p.peek()) {
		return atom, nil
	}
	start := p.pos
	switch p.src[p.pos] {
	case '*', '+', '?':
		p.pos++
	case '{':
		q, ok := p.braces()
		if !ok {
			return nil, unsupportedRegex("{ that is not a quantifier {n}, {n,} or {n,m}", `write \{ for a literal brace`)
		}
		p.pos += len(q)
	}
	if p.peek() == '?' {
		p.pos++ // lazy
	}
	if isQuantifierStart(p.peek()) {
		if p.peek() == '+' {
			return nil, unsupportedRegex("possessive quantifiers", "")
		}
		return nil, unsupportedRegex("nested quantifiers", "put the quantified part in a group")
	}
	if invalidQuantifierTarget(atom) {
		// Oniguruma rejects it: "target of repeat operator is invalid".
		return nil, p.errorf("quantifier on an anchor (^, $, \\A or \\z)")
	}
	return &reNode{kind: reRepeat, subs: []*reNode{atom}, text: p.src[start:p.pos]}, nil
}

// braces returns the quantifier {n}, {n,} or {n,m} at p.pos.
func (p *reParser) braces() (string, bool) {
	s := p.src[p.pos:]
	end := strings.IndexByte(s, '}')
	if end < 0 {
		return "", false
	}
	body := s[1:end]
	lo, hi, comma := strings.Cut(body, ",")
	if !allDigits(lo) || lo == "" || (comma && hi != "" && !allDigits(hi)) {
		return "", false
	}
	return s[:end+1], true
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

func (p *reParser) atom(depth int) (*reNode, error) {
	c := p.src[p.pos]
	switch c {
	case '(':
		p.pos++
		capture := true
		if strings.HasPrefix(p.src[p.pos:], "?") {
			switch {
			case strings.HasPrefix(p.src[p.pos:], "?:"):
				p.pos += 2
				capture = false
			case strings.HasPrefix(p.src[p.pos:], "?<=") || strings.HasPrefix(p.src[p.pos:], "?<!"):
				return nil, unsupportedRegex("lookbehind", "")
			case strings.HasPrefix(p.src[p.pos:], "?<"):
				end := strings.IndexByte(p.src[p.pos:], '>')
				if end < 0 || !validGroupName(p.src[p.pos+2:p.pos+end]) {
					return nil, unsupportedRegex("group names other than letters, digits and _", "")
				}
				p.pos += end + 1
			case strings.HasPrefix(p.src[p.pos:], "?=") || strings.HasPrefix(p.src[p.pos:], "?!"):
				return nil, unsupportedRegex("lookahead", "")
			case strings.HasPrefix(p.src[p.pos:], "?>"):
				return nil, unsupportedRegex("atomic groups", "")
			default:
				return nil, unsupportedRegex("inline flags and (? groups other than (?: and (?<name>", "")
			}
		}
		inner, err := p.alternation(depth + 1)
		if err != nil {
			return nil, err
		}
		if p.peek() != ')' {
			return nil, p.errorf("missing )")
		}
		p.pos++
		return &reNode{kind: reGroup, subs: []*reNode{inner}, capture: capture}, nil
	case '[':
		return p.class()
	case '.':
		p.pos++
		return &reNode{kind: reText, text: "."}, nil
	case '^':
		p.pos++
		return &reNode{kind: reBegin}, nil
	case '$':
		p.pos++
		return &reNode{kind: reEnd}, nil
	case '\\':
		return p.escape(false)
	case '*', '+', '?':
		return nil, p.errorf("missing argument to repetition operator %q", string(c))
	case '{':
		return nil, unsupportedRegex("{ that is not a quantifier {n}, {n,} or {n,m}", `write \{ for a literal brace`)
	case ']', '}':
		return nil, unsupportedRegex("unescaped "+string(c), `write \`+string(c)+" for the character")
	}
	r, size := utf8.DecodeRuneInString(p.src[p.pos:])
	p.pos += size
	return &reNode{kind: reText, text: quoteRune(r, false)}, nil
}

// invalidQuantifierTarget follows Oniguruma's is_invalid_quantifier_target:
// an anchor cannot be quantified, nor can a non-capturing group (which
// Oniguruma parses as its body) or an alternation that is or has one.
func invalidQuantifierTarget(n *reNode) bool {
	switch n.kind {
	case reBegin, reEnd, reZ:
		return true
	case reGroup:
		return !n.capture && invalidQuantifierTarget(n.subs[0])
	case reConcat:
		return len(n.subs) == 1 && invalidQuantifierTarget(n.subs[0])
	case reAlt:
		for _, s := range n.subs {
			if invalidQuantifierTarget(s) {
				return true
			}
		}
	}
	return false
}

func validGroupName(name string) bool {
	if name == "" || !isIdentStart(name[0]) {
		return false
	}
	for i := 1; i < len(name); i++ {
		if !isIdentChar(name[i]) {
			return false
		}
	}
	return true
}

// quoteRune returns r in Go regexp syntax, inside a class or not.
func quoteRune(r rune, inClass bool) string {
	switch {
	case r < 0x20 || r == 0x7f || !unicode.IsPrint(r):
		return fmt.Sprintf(`\x{%x}`, r)
	case r < utf8.RuneSelf && isPunct(byte(r)):
		if inClass && !strings.ContainsRune(`\]^-[`, r) {
			return string(r)
		}
		return `\` + string(r)
	}
	return string(r)
}

func isPunct(c byte) bool {
	return (c >= '!' && c <= '/') || (c >= ':' && c <= '@') || (c >= '[' && c <= '`') || (c >= '{' && c <= '~')
}

// escapeHint explains the escapes whose meaning differs between RE2 and
// Oniguruma.
var escapeHint = map[byte]string{
	'd': `in jq it matches any Unicode digit, in RE2 only 0-9; write [0-9] or \p{Nd}`,
	'D': `in jq it excludes every Unicode digit, in RE2 only 0-9; write [^0-9] or \P{Nd}`,
	'w': `in jq it matches any Unicode letter or digit, in RE2 only ASCII; write [A-Za-z0-9_] or a \p{...} class`,
	'W': `in jq it excludes every Unicode letter or digit, in RE2 only ASCII; write [^A-Za-z0-9_]`,
	's': `in jq it matches any Unicode space, in RE2 only ASCII; write [ \t\n\r\f] or \p{Z}`,
	'S': `in jq it excludes every Unicode space, in RE2 only ASCII`,
	'b': `jq's word boundaries are Unicode-aware, RE2's are ASCII-only`,
	'B': `jq's word boundaries are Unicode-aware, RE2's are ASCII-only`,
}

// escape parses a backslash escape outside or inside a class.
func (p *reParser) escape(inClass bool) (*reNode, error) {
	p.pos++ // backslash
	if p.pos >= len(p.src) {
		return nil, p.errorf("trailing backslash")
	}
	c := p.src[p.pos]
	p.pos++
	text := func(s string) (*reNode, error) { return &reNode{kind: reText, text: s}, nil }
	switch c {
	case 't':
		return text(`\t`)
	case 'n':
		return text(`\n`)
	case 'r':
		return text(`\r`)
	case 'f':
		return text(`\f`)
	case 'x':
		r, err := p.hexEscape()
		if err != nil {
			return nil, err
		}
		return text(quoteRune(r, inClass))
	case 'p', 'P':
		if !strings.HasPrefix(p.src[p.pos:], "{") {
			return nil, unsupportedRegex(`\`+string(c)+" without braces", `write \`+string(c)+"{L}")
		}
		end := strings.IndexByte(p.src[p.pos:], '}')
		if end < 0 {
			return nil, p.errorf(`missing } in \%c{...}`, c)
		}
		name := p.src[p.pos+1 : p.pos+end]
		p.pos += end + 1
		if !unicodeClassName(name) {
			return nil, unsupportedRegex(`\`+string(c)+"{"+name+"}", "use a Unicode general category (other than C) or a script, as in \\p{L} or \\p{Greek}")
		}
		return text(`\` + string(c) + "{" + name + "}")
	case 'A':
		if inClass {
			break
		}
		return &reNode{kind: reBegin}, nil
	case 'z':
		if inClass {
			break
		}
		return &reNode{kind: reZ}, nil
	}
	if hint, ok := escapeHint[c]; ok {
		return nil, unsupportedRegex(`\`+string(c), hint)
	}
	if c < utf8.RuneSelf && isPunct(c) && !strings.ContainsRune("<>`'", rune(c)) {
		return text(quoteRune(rune(c), inClass))
	}
	if isDigit(c) {
		return nil, unsupportedRegex(`\`+string(c)+" (backreferences and octal escapes)", "")
	}
	r, _ := utf8.DecodeRuneInString(p.src[p.pos-1:])
	return nil, unsupportedRegex(`\`+string(r), "")
}

// hexEscape parses the HH or {H...} after \x.
func (p *reParser) hexEscape() (rune, error) {
	s := p.src[p.pos:]
	var digits string
	if strings.HasPrefix(s, "{") {
		end := strings.IndexByte(s, '}')
		if end < 0 {
			return 0, p.errorf(`missing } in \x{...}`)
		}
		digits = s[1:end]
		p.pos += end + 1
	} else {
		if len(s) < 2 {
			return 0, unsupportedRegex(`\x with fewer than two hex digits`, `write \xHH or \x{H...}`)
		}
		digits = s[:2]
		p.pos += 2
	}
	n, err := strconv.ParseUint(digits, 16, 32)
	if err != nil || digits == "" || strings.ContainsAny(digits, "+-_xX") || n > unicode.MaxRune || (n >= 0xd800 && n < 0xe000) {
		return 0, unsupportedRegex(`\x`+digits, `write \xHH or \x{H...} with a valid code point`)
	}
	return rune(n), nil
}

// unicodeClassName reports whether name is a Unicode general category
// (other than C, whose meaning differs: jq's includes unassigned code
// points) or a script that Go's regexp knows.
func unicodeClassName(name string) bool {
	if name == "C" {
		return false
	}
	if name == "Any" {
		return true
	}
	if _, ok := unicode.Categories[name]; ok {
		return true
	}
	_, ok := unicode.Scripts[name]
	return ok
}

// class parses a bracket expression.
func (p *reParser) class() (*reNode, error) {
	p.pos++ // [
	var b strings.Builder
	b.WriteByte('[')
	if p.peek() == '^' {
		b.WriteByte('^')
		p.pos++
	}
	if p.peek() == ']' {
		return nil, unsupportedRegex("] at the start of a class", `write \] for the character`)
	}
	var items []classItem
	afterRange := false
	for {
		if p.pos >= len(p.src) {
			return nil, p.errorf("missing ]")
		}
		c := p.src[p.pos]
		switch {
		case c == ']':
			p.pos++
			for _, it := range items {
				b.WriteString(it.text)
			}
			b.WriteByte(']')
			return &reNode{kind: reText, text: b.String()}, nil
		case c == '[':
			return nil, unsupportedRegex("[ inside a class (nested classes and POSIX classes such as [:alpha:])", `write \[ for the character`)
		case c == '&' && strings.HasPrefix(p.src[p.pos:], "&&"):
			return nil, unsupportedRegex("&& inside a class (class intersection)", "")
		case c == '-' && len(items) > 0 && p.pos+1 < len(p.src) && p.src[p.pos+1] != ']':
			// A range: the previous item and the next one.
			prev := items[len(items)-1]
			if !prev.single || afterRange {
				return nil, unsupportedRegex("- after a range or a class escape inside a class", `write \- for the character`)
			}
			p.pos++
			next, err := p.classItem()
			if err != nil {
				return nil, err
			}
			if !next.single {
				return nil, unsupportedRegex("a range that ends in a class escape", "")
			}
			if next.r < prev.r {
				return nil, p.errorf("invalid range %s-%s", prev.text, next.text)
			}
			items[len(items)-1] = classItem{text: prev.text + "-" + next.text}
			afterRange = true
			continue
		}
		it, err := p.classItem()
		if err != nil {
			return nil, err
		}
		items = append(items, it)
		afterRange = false
	}
}

// classItem is one member of a bracket class.
type classItem struct {
	text   string // Go syntax
	r      rune   // the character, when single
	single bool   // a single character, which can start or end a range
}

// classItem parses one character or escape inside a class.
func (p *reParser) classItem() (classItem, error) {
	c := p.src[p.pos]
	if c == '\\' {
		start := p.pos
		n, err := p.escape(true)
		if err != nil {
			return classItem{}, err
		}
		esc := p.src[start:p.pos]
		if esc[1] == 'p' || esc[1] == 'P' {
			return classItem{text: n.text}, nil
		}
		r, ok := escapedRune(esc)
		return classItem{text: n.text, r: r, single: ok}, nil
	}
	if c == '[' {
		return classItem{}, unsupportedRegex("[ inside a class (nested classes and POSIX classes such as [:alpha:])", `write \[ for the character`)
	}
	r, size := utf8.DecodeRuneInString(p.src[p.pos:])
	p.pos += size
	return classItem{text: quoteRune(r, true), r: r, single: true}, nil
}

// escapedRune returns the character an escape such as \n, \x41 or \. stands
// for.
func escapedRune(esc string) (rune, bool) {
	switch esc {
	case `\t`:
		return '\t', true
	case `\n`:
		return '\n', true
	case `\r`:
		return '\r', true
	case `\f`:
		return '\f', true
	}
	if strings.HasPrefix(esc, `\x`) {
		digits := strings.Trim(esc[2:], "{}")
		n, err := strconv.ParseUint(digits, 16, 32)
		return rune(n), err == nil
	}
	r, _ := utf8.DecodeRuneInString(esc[1:])
	return r, true
}
