package filter

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/SCKelemen/clix/v2/internal/resource"
)

// Filter is a parsed --filter expression. It is safe for concurrent use:
// each Select keeps its own state.
type Filter struct {
	root node // nil selects everything
	loc  *time.Location
	now  func() time.Time
}

// node is an expression tree node.
type node interface{ isNode() }

type andNode struct{ left, right node }
type orNode struct{ left, right node }
type notNode struct{ expr node }

func (andNode) isNode()   {}
func (orNode) isNode()    {}
func (notNode) isNode()   {}
func (*termNode) isNode() {}

// op is a term operator.
type op int

const (
	opHas op = iota // :
	opEQ            // =
	opNE            // !=
	opLT            // <
	opLE            // <=
	opGT            // >
	opGE            // >=
)

var opNames = map[string]op{":": opHas, "=": opEQ, "!=": opNE, "<": opLT, "<=": opLE, ">": opGT, ">=": opGE}

func (o op) String() string {
	for name, v := range opNames {
		if v == o {
			return name
		}
	}
	return "?"
}

// wordMatch reports whether o matches words (":", "=" and "!=") rather
// than comparing.
func (o op) wordMatch() bool { return o == opHas || o == opEQ || o == opNE }

// termNode is a key operator operand term.
type termNode struct {
	key      resource.Key
	op       op
	operands []*operand // one, or the list of key:(a b) and key=(a b)
	patterns []pattern  // for ":", "=" and "!=": one per operand
	zone     bool       // the key is zone or region alone (see matchOne)
	list     bool       // the operands were written as a list, key:(a b)
}

// operand is gcloud's _ExprOperand for one value.
type operand struct {
	text     string
	num      *resource.Number // the numeric value, when there is one
	constant bool             // true or false, which are also 1 and 0
}

// pattern is gcloud's (operand, standard_regex, deprecated_regex) triple.
// The deprecated pattern decides a match when there is one; gcloud has
// none for the ":" pattern "*", whose standard pattern "." matches any
// character but a newline (any is set), and for a ":" pattern that is
// empty once normalized, whose standard pattern \b\b matches a word
// boundary, so any text with a word character (word is set).
type pattern struct {
	text string
	re   *regexp.Regexp
	any  bool
	word bool
}

// Option configures Parse.
type Option func(*Filter)

// WithLocation sets the time zone of dates and times without one, which
// gcloud reads in the local time zone (time.Local, the default).
func WithLocation(loc *time.Location) Option {
	return func(f *Filter) { f.loc = loc }
}

// Parse parses a --filter expression. Syntax errors are
// *resource.SyntaxError values with gcloud's messages; what gcloud
// accepts but clix does not implement is a *resource.UnsupportedError.
// An empty expression selects everything.
func Parse(expr string, opts ...Option) (*Filter, error) {
	p := &parser{lex: resource.NewLexer(expr), parens: []map[op]bool{{}}}
	f := &Filter{loc: time.Local, now: time.Now}
	for _, o := range opts {
		o(f)
	}
	root, err := p.expr(false)
	if err != nil {
		return nil, unsupportedIn(err)
	}
	if !p.lex.EndOfInput() {
		return nil, &resource.SyntaxError{Msg: "Unexpected tokens [" + p.lex.Annotate(p.lex.Position()) + "] in expression."}
	}
	f.root = root
	return f, nil
}

// unsupportedIn marks an *UnsupportedError as one of --filter.
func unsupportedIn(err error) error {
	var u *resource.UnsupportedError
	if errors.As(err, &u) && u.In == "" {
		u.In = "--filter"
	}
	return err
}

func unsupported(format string, args ...any) error {
	err := resource.Unsupportedf(format, args...)
	return unsupportedIn(err)
}

// parser is gcloud's resource_filter._Parser.
type parser struct {
	lex    *resource.Lexer
	parens []map[op]bool // the AND and OR seen in each parenthesis group
}

var logical = []string{"AND", "NOT", "OR"}

// checkParens is _CheckParenthesization: AND and OR may not both appear
// in one parenthesis group.
func (p *parser) checkParens(o op) error {
	group := p.parens[len(p.parens)-1]
	group[o] = true
	if len(group) > 1 {
		return p.lex.Errorf("Parenthesis grouping is required when AND and OR are are combined")
	}
	return nil
}

// Logical operators reuse the op type as set keys.
const (
	parenAND op = 100
	parenOR  op = 101
)

func (p *parser) termExpected() error { return p.lex.Errorf("Term expected") }

// expr: adjterm adjtail. Juxtaposed terms are joined with AND, the lowest
// precedence.
func (p *parser) expr(must bool) (node, error) {
	tree, err := p.adjTerm(false)
	if err != nil {
		return nil, err
	}
	if tree == nil {
		if must {
			return nil, p.termExpected()
		}
		return nil, nil
	}
	return p.adjTail(tree)
}

func (p *parser) adjTail(tree node) (node, error) {
	if !p.lex.IsString("AND", true) && !p.lex.IsString("OR", true) && !p.peek(")") && !p.lex.EndOfInput() {
		right, err := p.expr(true)
		if err != nil {
			return nil, err
		}
		return andNode{tree, right}, nil
	}
	return tree, nil
}

// adjterm: orterm ortail.
func (p *parser) adjTerm(must bool) (node, error) {
	tree, err := p.orTerm(false)
	if err != nil {
		return nil, err
	}
	if tree == nil {
		if must {
			return nil, p.termExpected()
		}
		return nil, nil
	}
	if p.lex.IsString("OR", false) {
		if err := p.checkParens(parenOR); err != nil {
			return nil, err
		}
		right, err := p.adjTerm(true)
		if err != nil {
			return nil, err
		}
		return orNode{tree, right}, nil
	}
	return tree, nil
}

// orterm: andterm andtail.
func (p *parser) orTerm(must bool) (node, error) {
	tree, err := p.andTerm(false)
	if err != nil {
		return nil, err
	}
	if tree == nil {
		if must {
			return nil, p.termExpected()
		}
		return nil, nil
	}
	if p.lex.IsString("AND", false) {
		if err := p.checkParens(parenAND); err != nil {
			return nil, err
		}
		right, err := p.orTerm(true)
		if err != nil {
			return nil, err
		}
		return andNode{tree, right}, nil
	}
	return tree, nil
}

// andterm: term | NOT term.
func (p *parser) andTerm(must bool) (node, error) {
	if p.lex.IsString("NOT", false) {
		t, err := p.term(true)
		if err != nil {
			return nil, err
		}
		if t == nil {
			// gcloud fails evaluating NOT of an empty group.
			return nil, p.termExpected()
		}
		return notNode{t}, nil
	}
	return p.term(must)
}

func (p *parser) takeChar(chars string) bool {
	c, _ := p.lex.IsCharacter(chars, false, true)
	return c != 0
}

func (p *parser) peek(chars string) bool {
	c, _ := p.lex.IsCharacter(chars, true, false)
	return c != 0
}

// term: [-]key operator operand | ( expr ).
func (p *parser) term(must bool) (node, error) {
	here := p.lex.Position()
	if ok, _ := p.lex.SkipSpace("", ""); !ok {
		if must {
			return nil, p.lex.ErrorAt(here, "Term expected")
		}
		return nil, nil
	}
	if p.peek(")") {
		return nil, nil
	}
	if c, _ := p.lex.IsCharacter("(", false, false); c != 0 {
		p.parens = append(p.parens, map[op]bool{})
		tree, err := p.expr(false)
		if err != nil {
			return nil, err
		}
		if _, err := p.lex.IsCharacter(")", false, false); err != nil {
			return nil, err
		}
		p.parens = p.parens[:len(p.parens)-1]
		return tree, nil
	}
	invert, _ := p.lex.IsCharacter("-", false, false)

	here = p.lex.Position()
	key, err := p.lex.Key()
	if err != nil {
		return nil, err
	}
	if len(key) > 0 && key[0].IsName() && contains(logical, key[0].Name) {
		return nil, p.lex.ErrorAt(here, "Term expected")
	}
	if c, _ := p.lex.IsCharacter("(", false, true); c != 0 {
		return nil, unsupported("calling a transform such as %s() in a term", key.String())
	}

	o, found, err := p.operator()
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, unsupported("a term without an operator (%q, a gcloud global restriction)", key.String())
	}
	if _, err := p.lex.SkipSpace("Operand", ""); err != nil {
		return nil, err
	}
	here = p.lex.Position()
	for _, l := range logical {
		if p.lex.IsString(l, false) {
			return nil, p.lex.ErrorAt(here, "Logical operator not expected")
		}
	}
	t := &termNode{key: key, op: o}
	t.zone = len(key) == 1 && key[0].IsName() && (key[0].Name == "zone" || key[0].Name == "region")
	if (o == opEQ || o == opHas) && p.takeChar("(") {
		t.list = true
		args, err := p.lex.Args(" \t\n,")
		if err != nil {
			return nil, err
		}
		for _, a := range args {
			if !contains(logical, a.Text) {
				t.operands = append(t.operands, nil)
				t.operands[len(t.operands)-1], err = newOperand(a.Text)
				if err != nil {
					return nil, err
				}
			}
		}
		if len(t.operands) == 0 {
			return nil, unsupported("an empty operand list %s()", o)
		}
	} else {
		tok, err := p.lex.Token("()", false, true)
		if err != nil {
			return nil, err
		}
		if !tok.Present {
			return nil, p.lex.ErrorAt(here, "Term operand expected")
		}
		opd, err := newOperand(tok.Text)
		if err != nil {
			return nil, err
		}
		t.operands = []*operand{opd}
	}
	if o.wordMatch() {
		for _, opd := range t.operands {
			pat, err := newPattern(o, opd.text)
			if err != nil {
				return nil, err
			}
			t.patterns = append(t.patterns, pat)
		}
	}
	if invert != 0 {
		return notNode{t}, nil
	}
	return t, nil
}

// operator reads an operator (_ParseOperator): one of the characters
// :=!<>~ followed by an optional second one of :=!<>~ that makes a known
// operator.
func (p *parser) operator() (op, bool, error) {
	if ok, _ := p.lex.SkipSpace("", ""); !ok {
		return 0, false, nil
	}
	here := p.lex.Position()
	const first = ":=!<>~"
	c, _ := p.lex.IsCharacter(first, false, false)
	if c == 0 {
		return 0, false, nil
	}
	name := string(c)
	if !p.lex.EndOfInput() {
		if c2, _ := p.lex.IsCharacter(first+"=~", false, false); c2 != 0 {
			name += string(c2)
		}
	}
	if name == "~" || name == "!~" {
		return 0, false, unsupported("the %s operator (a Python regular expression)", name)
	}
	o, ok := opNames[name]
	if !ok {
		return 0, false, p.lex.ErrorAt(here, "Malformed operator")
	}
	if _, err := p.lex.SkipSpace("Term operand", ""); err != nil {
		return 0, false, err
	}
	return o, true, nil
}

// newOperand is _ExprOperand.Initialize for a string: true and false are
// the numeric constants 1 and 0; otherwise the number int() or float()
// reads, if any.
func newOperand(text string) (*operand, error) {
	o := &operand{text: text}
	switch strings.ToLower(text) {
	case "true":
		o.num, o.constant = numberPtr(resource.NumberOf(true)), true
		return o, nil
	case "false":
		o.num, o.constant = numberPtr(resource.NumberOf(false)), true
		return o, nil
	}
	n, ok, err := resource.NumericType(text)
	if err != nil {
		return nil, err
	}
	if ok {
		o.num = &n
	}
	return o, nil
}

func numberPtr(n resource.Number, _ bool) *resource.Number { return &n }

// newPattern builds the pattern of a ":", "=" or "!=" operand: gcloud's
// _ExprHAS._AddPattern and _ExprEQ._AddPattern, keeping the deprecated
// pattern, which is the one that decides matches in gcloud today.
func newPattern(o op, text string) (pattern, error) {
	const flags = "(?im)" // re.IGNORECASE | re.MULTILINE (| re.UNICODE)
	normalized := resource.NormalizeForSearch(text, false)
	if o != opHas {
		return pattern{text: text, re: regexp.MustCompile(flags + "^" + regexp.QuoteMeta(normalized) + "$")}, nil
	}
	if text == "*" {
		return pattern{text: text, any: true}, nil
	}
	parts := strings.Split(normalized, "*")
	if len(parts) > 2 {
		return pattern{}, &resource.SyntaxError{Msg: "At most one * expected in : patterns [" + text + "]."}
	}
	head, glob, tail := true, "", true
	if strings.HasSuffix(normalized, "*") {
		tail = false
	}
	switch {
	case len(parts) == 1:
		parts = append(parts, "")
	case strings.HasPrefix(text, "*"):
		head = false
	case strings.HasSuffix(text, "*"):
		tail = false
	default:
		glob = ".*"
	}
	left, right := regexp.QuoteMeta(parts[0]), regexp.QuoteMeta(parts[1])
	var expr string
	switch {
	case head && tail && glob != "":
		expr = "^" + left + glob + right + "$"
	case head && tail && left+right == "":
		return pattern{text: text, word: true}, nil
	case head && tail:
		expr = left + right
	case head:
		expr = "^" + left + glob + right
	case tail:
		expr = left + glob + right + "$"
	default:
		return pattern{}, unsupported("the : pattern %q, which is * alone once normalized,", text)
	}
	return pattern{text: text, re: regexp.MustCompile(flags + expr)}, nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
