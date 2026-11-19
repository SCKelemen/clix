package jq

import (
	"fmt"
	"regexp"
	"strings"
)

// SyntaxError is a filter that does not parse.
type SyntaxError struct {
	Offset int // byte offset in the filter
	Msg    string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("%s (at offset %d)", e.Msg, e.Offset)
}

// UnsupportedError is a valid jq construct outside the subset clix
// implements.
type UnsupportedError struct {
	Offset    int    // byte offset in the filter, or -1 (a regex checked while running)
	Construct string // such as "the + operator" or "the function splits/1"
	Hint      string // optional advice
}

// pluralConstructs start the names of constructs that take "are".
var pluralConstructs = []string{
	"slices", "variables", "variable bindings", "function definitions", "modules",
	"format strings", "format keys", "regex flags", "regex anchors", "regex possessive",
	"regex nested", "regex group names", "regex atomic", "regex inline flags",
}

func (e *UnsupportedError) Error() string {
	verb := " is"
	for _, p := range pluralConstructs {
		if strings.HasPrefix(e.Construct, p) {
			verb = " are"
		}
	}
	msg := e.Construct + verb + " not supported in clix --jq"
	if e.Offset >= 0 {
		msg += fmt.Sprintf(" (at offset %d)", e.Offset)
	}
	if e.Hint != "" {
		msg += ": " + e.Hint
	}
	return msg
}

// The filter's syntax tree.
type node interface{}

type (
	identityNode struct{}
	recurseNode  struct{}
	literalNode  struct{ v any }
	// negatedNode is a negative number literal such as -1: a computed
	// number, and not a constant for jq's compile-time checks.
	negatedNode struct{ v number }
	// indexNode is target[key]; key is evaluated with the same input as
	// target, as in jq. With opt (a "?" right after it), an index that
	// fails gives no output and the next target or key is tried.
	indexNode struct {
		target, key node
		opt         bool
	}
	// iterateNode is target[], or target[]? with opt.
	iterateNode struct {
		target node
		opt    bool
	}
	// tryNode is "e?" after anything but an index: the errors of e end
	// its outputs silently.
	tryNode   struct{ body node }
	pipeNode  struct{ left, right node }
	commaNode struct{ left, right node }
	altNode   struct{ left, right node }
	andNode   struct{ left, right node }
	orNode    struct{ left, right node }
	cmpNode   struct {
		op          string
		left, right node
	}
	arrayNode  struct{ body node } // body is nil for []
	objectNode struct{ entries []objectEntry }
	stringNode struct{ parts []strPart }
	ifNode     struct {
		cond, then node
		els        node // nil means identity
	}
	callNode struct {
		name string
		args []node
		re   *regexp.Regexp // test/1 with a constant pattern
	}
	formatNode struct{ name string }
)

type objectEntry struct {
	key, value node
	shorthand  bool // {"\(k)"}: the value is .[key]
}

// builtins lists the supported functions by arity.
var builtins = map[string][]int{
	"empty": {0}, "not": {0}, "length": {0}, "keys": {0}, "keys_unsorted": {0},
	"to_entries": {0}, "tostring": {0}, "tojson": {0}, "ascii_downcase": {0},
	"ascii_upcase": {0}, "type": {0}, "values": {0}, "sort": {0},
	"first": {0, 1}, "last": {0, 1},
	"select": {1}, "map": {1}, "has": {1}, "join": {1}, "test": {1}, "sort_by": {1},
}

// formats lists the supported @format filters.
var formats = map[string]bool{"text": true, "json": true, "csv": true, "tsv": true}

type parser struct {
	lex   lexer
	tok   token
	depth int // nesting of the filter being parsed
}

// maxNesting bounds how deeply a filter may nest (parentheses, brackets,
// braces and string interpolations), so a hostile filter cannot exhaust
// the stack.
const maxNesting = 1000

// parse parses a whole filter.
func parse(src string) (node, error) {
	return parseSub(src, 0, 0)
}

// parseSub parses src, which starts at offset base of the whole filter
// (base > 0 for a string interpolation) at nesting depth.
func parseSub(src string, base, depth int) (node, error) {
	p := &parser{lex: lexer{src: src, base: base, depth: depth}, depth: depth}
	if err := p.advance(); err != nil {
		return nil, err
	}
	if p.tok.kind == tEOF {
		if base > 0 {
			return nil, p.errorf("empty string interpolation")
		}
		return nil, p.errorf(`empty filter (write "." for the input as is)`)
	}
	n, err := p.pipe()
	if err != nil {
		return nil, err
	}
	if p.tok.kind != tEOF {
		return nil, p.unexpected()
	}
	return n, nil
}

func (p *parser) advance() error {
	t, err := p.lex.next()
	if err != nil {
		return err
	}
	p.tok = t
	return nil
}

func (p *parser) errorf(format string, args ...any) error {
	return &SyntaxError{Offset: p.lex.base + p.tok.pos, Msg: fmt.Sprintf(format, args...)}
}

func (p *parser) unsupported(construct, hint string) error {
	return &UnsupportedError{Offset: p.lex.base + p.tok.pos, Construct: construct, Hint: hint}
}

func (p *parser) unexpected() error {
	switch p.tok.kind {
	case tEOF:
		return p.errorf("unexpected end of filter")
	case tString:
		return p.errorf("unexpected string")
	case tOp, tIdent, tNumber:
		return p.errorf("unexpected %q", p.tok.text)
	case tField:
		return p.errorf("unexpected %q", "."+p.tok.text)
	}
	return p.errorf("unexpected token")
}

func (p *parser) isOp(text string) bool { return p.tok.kind == tOp && p.tok.text == text }

func (p *parser) isKeyword(text string) bool { return p.tok.kind == tIdent && p.tok.text == text }

func (p *parser) expectOp(text string) error {
	if !p.isOp(text) {
		if p.tok.kind == tEOF {
			return p.errorf("expected %q, found the end of the filter", text)
		}
		return p.errorf("expected %q", text)
	}
	return p.advance()
}

func (p *parser) expectKeyword(text string) error {
	if !p.isKeyword(text) {
		return p.errorf("expected %q", text)
	}
	return p.advance()
}

// pipe parses the lowest-precedence level: a | b (right-associative).
func (p *parser) pipe() (node, error) {
	p.depth++
	p.lex.depth = p.depth
	defer func() {
		p.depth--
		p.lex.depth = p.depth
	}()
	if p.depth > maxNesting {
		return nil, p.errorf("the filter nests more than %d levels deep", maxNesting)
	}
	left, err := p.comma()
	if err != nil {
		return nil, err
	}
	if p.isOp("|") {
		if err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.pipe()
		if err != nil {
			return nil, err
		}
		return pipeNode{left, right}, nil
	}
	return left, nil
}

func (p *parser) comma() (node, error) {
	left, err := p.alt()
	if err != nil {
		return nil, err
	}
	for p.isOp(",") {
		if err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.alt()
		if err != nil {
			return nil, err
		}
		left = commaNode{left, right}
	}
	return left, nil
}

// alt parses a // b (right-associative).
func (p *parser) alt() (node, error) {
	left, err := p.or()
	if err != nil {
		return nil, err
	}
	if p.isOp("//") {
		if err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.alt()
		if err != nil {
			return nil, err
		}
		return altNode{left, right}, nil
	}
	return left, nil
}

func (p *parser) or() (node, error) {
	left, err := p.and()
	if err != nil {
		return nil, err
	}
	for p.isKeyword("or") {
		if err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.and()
		if err != nil {
			return nil, err
		}
		left = orNode{left, right}
	}
	return left, nil
}

func (p *parser) and() (node, error) {
	left, err := p.comparison()
	if err != nil {
		return nil, err
	}
	for p.isKeyword("and") {
		if err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.comparison()
		if err != nil {
			return nil, err
		}
		left = andNode{left, right}
	}
	return left, nil
}

func isComparison(t token) bool {
	if t.kind != tOp {
		return false
	}
	switch t.text {
	case "==", "!=", "<", "<=", ">", ">=":
		return true
	}
	return false
}

// comparison parses a == b and the other comparisons, which do not
// associate: a == b == c is a syntax error, as in jq.
func (p *parser) comparison() (node, error) {
	left, err := p.operand()
	if err != nil {
		return nil, err
	}
	if !isComparison(p.tok) {
		return left, nil
	}
	op := p.tok.text
	if err := p.advance(); err != nil {
		return nil, err
	}
	right, err := p.operand()
	if err != nil {
		return nil, err
	}
	if isComparison(p.tok) {
		return nil, p.errorf("unexpected %q: comparisons do not chain", p.tok.text)
	}
	return cmpNode{op, left, right}, nil
}

// operand parses one operand of a binary operator and rejects the
// operators clix does not implement when they follow it.
func (p *parser) operand() (node, error) {
	n, err := p.unary()
	if err != nil {
		return nil, err
	}
	if err := p.rejectOperator(); err != nil {
		return nil, err
	}
	return n, nil
}

// rejectOperator fails on an arithmetic, assignment or binding operator.
func (p *parser) rejectOperator() error {
	if p.tok.kind == tOp {
		switch p.tok.text {
		case "+", "-", "*", "/", "%":
			return p.unsupported("the "+p.tok.text+" operator", "clix --jq has no arithmetic")
		case "=", "|=", "+=", "-=", "*=", "/=", "%=", "//=":
			return p.unsupported("the "+p.tok.text+" operator", "clix --jq does not update values")
		case "?//":
			return p.unsupported("the ?// operator", "")
		}
	}
	if p.isKeyword("as") {
		return p.unsupported("variable bindings (... as $x)", "")
	}
	return nil
}

// unary parses an operand with an optional minus sign, which clix allows
// only in front of a number literal.
func (p *parser) unary() (node, error) {
	if !p.isOp("-") {
		return p.postfix()
	}
	if err := p.advance(); err != nil {
		return nil, err
	}
	n, err := p.postfix()
	if err != nil {
		return nil, err
	}
	return negate(p, n)
}

// negate returns the negation of a number literal, a computed number as in
// jq ("-1.0" prints "-1").
func negate(p *parser, n node) (node, error) {
	switch x := n.(type) {
	case literalNode:
		if num, ok := x.v.(number); ok {
			return negatedNode{computed(-num.f)}, nil
		}
	case negatedNode:
		return negatedNode{computed(-x.v.f)}, nil
	}
	return nil, p.unsupported("negation (unary -)", "only a negative number literal such as -1 is supported")
}

// postfix parses a term, an if expression, and any "?" after them.
func (p *parser) postfix() (node, error) {
	var n node
	var err error
	if p.tok.kind == tIdent {
		switch p.tok.text {
		case "if":
			n, err = p.ifExpr()
		case "reduce", "foreach":
			return nil, p.unsupported(p.tok.text, "")
		case "try", "catch":
			return nil, p.unsupported("try/catch", `use the postfix "?" to ignore errors`)
		case "label":
			return nil, p.unsupported("label/break", "")
		case "def":
			return nil, p.unsupported("function definitions (def)", "")
		case "import", "include":
			return nil, p.unsupported("modules ("+p.tok.text+")", "")
		}
	}
	if n == nil && err == nil {
		n, err = p.term()
	}
	if err != nil {
		return nil, err
	}
	for p.isOp("?") {
		if err := p.advance(); err != nil {
			return nil, err
		}
		n = tryNode{n}
	}
	return n, nil
}

func (p *parser) ifExpr() (node, error) {
	if err := p.advance(); err != nil { // if
		return nil, err
	}
	cond, err := p.pipe()
	if err != nil {
		return nil, err
	}
	if err := p.expectKeyword("then"); err != nil {
		return nil, err
	}
	then, err := p.pipe()
	if err != nil {
		return nil, err
	}
	n := &ifNode{cond: cond, then: then}
	switch {
	case p.isKeyword("elif"):
		// "elif" starts a nested if that shares the "end".
		els, err := p.ifExpr()
		if err != nil {
			return nil, err
		}
		n.els = els
		return *n, nil
	case p.isKeyword("else"):
		if err := p.advance(); err != nil {
			return nil, err
		}
		els, err := p.pipe()
		if err != nil {
			return nil, err
		}
		n.els = els
		if err := p.expectKeyword("end"); err != nil {
			return nil, err
		}
	case p.isKeyword("end"):
		if err := p.advance(); err != nil {
			return nil, err
		}
	default:
		return nil, p.errorf(`expected "elif", "else" or "end"`)
	}
	return *n, nil
}

// term parses a term and its suffixes (.name, ."name", [e], [], and a "?"
// right after one of them).
func (p *parser) term() (node, error) {
	n, indexed, err := p.primary()
	if err != nil {
		return nil, err
	}
	for {
		if indexed && p.isOp("?") {
			if err := p.advance(); err != nil {
				return nil, err
			}
			n = optional(n)
		}
		indexed = true
		switch {
		case p.tok.kind == tField:
			n = indexNode{target: n, key: literalNode{p.tok.text}}
			if err := p.advance(); err != nil {
				return nil, err
			}
		case p.tok.kind == tDot:
			if err := p.advance(); err != nil {
				return nil, err
			}
			switch {
			case p.tok.kind == tString:
				key := stringLiteral(p.tok.parts)
				if err := p.advance(); err != nil {
					return nil, err
				}
				n = indexNode{target: n, key: key}
			case p.isOp("["):
				if n, err = p.bracket(n); err != nil {
					return nil, err
				}
			default:
				return nil, p.unexpected()
			}
		case p.isOp("["):
			if n, err = p.bracket(n); err != nil {
				return nil, err
			}
		default:
			return n, nil
		}
	}
}

// optional marks the index or iteration n as optional ("?" right after
// it), as jq's INDEX_OPT and EACH_OPT do.
func optional(n node) node {
	switch x := n.(type) {
	case indexNode:
		x.opt = true
		return x
	case iterateNode:
		x.opt = true
		return x
	}
	panic("jq: optional on a node that is not an index")
}

// bracket parses [e] or [] after target.
func (p *parser) bracket(target node) (node, error) {
	if err := p.advance(); err != nil { // [
		return nil, err
	}
	if p.isOp("]") {
		return iterateNode{target: target}, p.advance()
	}
	if p.isOp(":") {
		return nil, p.unsupported("slices (.[a:b])", "")
	}
	key, err := p.pipe()
	if err != nil {
		return nil, err
	}
	if p.isOp(":") {
		return nil, p.unsupported("slices (.[a:b])", "")
	}
	if err := p.expectOp("]"); err != nil {
		return nil, err
	}
	return indexNode{target: target, key: key}, nil
}

// stringLiteral turns lexed string parts into a node: a literal when there
// is no interpolation.
func stringLiteral(parts []strPart) node {
	if len(parts) == 1 && parts[0].expr == nil {
		return literalNode{parts[0].text}
	}
	return stringNode{parts}
}

// primary parses the start of a term. indexed reports whether it was an
// index (.name or ."name"), after which a "?" belongs to the term.
func (p *parser) primary() (n node, indexed bool, err error) {
	t := p.tok
	switch t.kind {
	case tDot:
		if err := p.advance(); err != nil {
			return nil, false, err
		}
		if p.tok.kind == tString {
			key := stringLiteral(p.tok.parts)
			return indexNode{target: identityNode{}, key: key}, true, p.advance()
		}
		return identityNode{}, false, nil
	case tRec:
		return recurseNode{}, false, p.advance()
	case tField:
		return indexNode{target: identityNode{}, key: literalNode{t.text}}, true, p.advance()
	case tNumber:
		d, ok := parseDecimal(t.text)
		if !ok {
			return nil, false, p.errorf("invalid number literal %q", t.text)
		}
		return literalNode{literalNumber(d)}, false, p.advance()
	case tString:
		return stringLiteral(t.parts), false, p.advance()
	case tFormat:
		if err := p.advance(); err != nil {
			return nil, false, err
		}
		if p.tok.kind == tString {
			return nil, false, &UnsupportedError{Offset: p.lex.base + t.pos, Construct: "format strings (@" + t.text + ` "...")`}
		}
		if !formats[t.text] {
			return nil, false, &UnsupportedError{Offset: p.lex.base + t.pos, Construct: "the format @" + t.text, Hint: "supported formats are @text, @json, @csv and @tsv"}
		}
		return formatNode{t.text}, false, nil
	case tVar:
		if t.text == "__loc__" {
			return nil, false, p.unsupported("$__loc__", "")
		}
		return nil, false, p.unsupported("variables ($"+t.text+")", "")
	case tOp:
		switch t.text {
		case "(":
			if err := p.advance(); err != nil {
				return nil, false, err
			}
			e, err := p.pipe()
			if err != nil {
				return nil, false, err
			}
			return e, false, p.expectOp(")")
		case "[":
			if err := p.advance(); err != nil {
				return nil, false, err
			}
			if p.isOp("]") {
				return arrayNode{}, false, p.advance()
			}
			e, err := p.pipe()
			if err != nil {
				return nil, false, err
			}
			return arrayNode{e}, false, p.expectOp("]")
		case "{":
			n, err := p.object()
			return n, false, err
		}
	case tIdent:
		return p.call()
	}
	return nil, false, p.unexpected()
}

// call parses a function call or true, false and null.
func (p *parser) call() (node, bool, error) {
	t := p.tok
	if keywords[t.text] {
		return nil, false, p.unexpected()
	}
	if err := p.advance(); err != nil {
		return nil, false, err
	}
	var args []node
	if p.isOp("(") {
		for {
			if err := p.advance(); err != nil {
				return nil, false, err
			}
			arg, err := p.pipe()
			if err != nil {
				return nil, false, err
			}
			args = append(args, arg)
			if p.isOp(";") {
				continue
			}
			if err := p.expectOp(")"); err != nil {
				return nil, false, err
			}
			break
		}
	}
	if len(args) == 0 {
		switch t.text {
		case "true":
			return literalNode{true}, false, nil
		case "false":
			return literalNode{false}, false, nil
		case "null":
			return literalNode{nil}, false, nil
		}
	}
	supported := false
	for _, arity := range builtins[t.text] {
		supported = supported || arity == len(args)
	}
	if !supported {
		name := fmt.Sprintf("the function %s/%d", t.text, len(args))
		hint := ""
		if t.text == "test" && len(args) == 2 {
			hint = "regex flags are not supported, because jq's case folding differs from Go's"
		}
		return nil, false, &UnsupportedError{Offset: p.lex.base + t.pos, Construct: name, Hint: hint}
	}
	c := callNode{name: t.text, args: args}
	if c.name == "test" {
		if lit, ok := args[0].(literalNode); ok {
			if pattern, ok := lit.v.(string); ok {
				re, err := compileRegex(pattern)
				if err != nil {
					if u, ok := err.(*UnsupportedError); ok {
						u.Offset = p.lex.base + t.pos
						return nil, false, u
					}
					return nil, false, &SyntaxError{Offset: p.lex.base + t.pos, Msg: err.Error()}
				}
				c.re = re
			}
		}
	}
	return c, false, nil
}

// object parses {...}.
func (p *parser) object() (node, error) {
	if err := p.advance(); err != nil { // {
		return nil, err
	}
	var entries []objectEntry
	for !p.isOp("}") {
		e, err := p.objectEntry()
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
		if p.isOp(",") {
			if err := p.advance(); err != nil {
				return nil, err
			}
			continue
		}
		if !p.isOp("}") {
			return nil, p.errorf(`expected "," or "}"`)
		}
	}
	return objectNode{entries}, p.advance()
}

func (p *parser) objectEntry() (objectEntry, error) {
	t := p.tok
	switch t.kind {
	case tIdent:
		if err := p.advance(); err != nil {
			return objectEntry{}, err
		}
		key := literalNode{t.text}
		if !p.isOp(":") {
			return objectEntry{key: key, value: indexNode{target: identityNode{}, key: key}}, nil
		}
		v, err := p.objectValue()
		return objectEntry{key: key, value: v}, err
	case tString:
		if err := p.advance(); err != nil {
			return objectEntry{}, err
		}
		key := stringLiteral(t.parts)
		if !p.isOp(":") {
			return objectEntry{key: key, shorthand: true}, nil
		}
		v, err := p.objectValue()
		return objectEntry{key: key, value: v}, err
	case tVar:
		if t.text == "__loc__" {
			return objectEntry{}, p.unsupported("$__loc__", "")
		}
		return objectEntry{}, p.unsupported("variables ($"+t.text+")", "")
	case tFormat:
		return objectEntry{}, p.unsupported("format keys (@"+t.text+")", "")
	case tOp:
		if t.text == "(" {
			if err := p.advance(); err != nil {
				return objectEntry{}, err
			}
			key, err := p.pipe()
			if err != nil {
				return objectEntry{}, err
			}
			if err := p.expectOp(")"); err != nil {
				return objectEntry{}, err
			}
			// jq rejects a constant key that is not a string when it
			// compiles the filter, even where it would never run.
			if v, ok := constant(key); ok {
				if _, isString := v.(string); !isString {
					return objectEntry{}, &SyntaxError{Offset: p.lex.base + t.pos, Msg: "Cannot use " + describe(v) + " as object key"}
				}
			}
			if !p.isOp(":") {
				return objectEntry{}, p.errorf(`expected ":"`)
			}
			v, err := p.objectValue()
			return objectEntry{key: key, value: v}, err
		}
	}
	return objectEntry{}, p.unexpected()
}

// constant returns the value of n when jq treats it as a constant: a
// literal (but not a negated number), an array or object built only from
// constants, a comparison of constants (which jq folds), or a constant
// piped into or from ".".
func constant(n node) (any, bool) {
	switch x := n.(type) {
	case literalNode:
		return x.v, true
	case pipeNode:
		if _, ok := x.left.(identityNode); ok {
			return constant(x.right)
		}
		if _, ok := x.right.(identityNode); ok {
			return constant(x.left)
		}
	case cmpNode:
		l, ok := constant(x.left)
		if !ok {
			return nil, false
		}
		r, ok := constant(x.right)
		if !ok {
			return nil, false
		}
		return compareOp(x.op, l, r), true
	case arrayNode:
		arr := []any{}
		var collect func(n node) bool
		collect = func(n node) bool {
			if c, ok := n.(commaNode); ok {
				return collect(c.left) && collect(c.right)
			}
			v, ok := constant(n)
			arr = append(arr, v)
			return ok
		}
		if x.body != nil && !collect(x.body) {
			return nil, false
		}
		return arr, true
	case objectNode:
		obj := newObject(len(x.entries))
		for _, e := range x.entries {
			if e.shorthand {
				return nil, false
			}
			k, ok := constant(e.key)
			key, isString := k.(string)
			if !ok || !isString {
				return nil, false
			}
			v, ok := constant(e.value)
			if !ok {
				return nil, false
			}
			obj.set(key, v)
		}
		return obj, true
	}
	return nil, false
}

// objectValue parses ": value". jq allows only terms joined by "|" here
// (and a leading "-"), so {a: .x == 1} is a syntax error, as in jq.
func (p *parser) objectValue() (node, error) {
	if err := p.advance(); err != nil { // :
		return nil, err
	}
	left, err := p.objectTerm()
	if err != nil {
		return nil, err
	}
	for p.isOp("|") {
		if err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.objectTerm()
		if err != nil {
			return nil, err
		}
		left = pipeNode{left, right}
	}
	return left, nil
}

func (p *parser) objectTerm() (node, error) {
	if p.isOp("-") {
		if err := p.advance(); err != nil {
			return nil, err
		}
		n, err := p.objectTerm()
		if err != nil {
			return nil, err
		}
		return negate(p, n)
	}
	if p.isKeyword("if") {
		return nil, p.errorf(`unexpected "if": write (if ... end) in an object value`)
	}
	return p.term()
}
