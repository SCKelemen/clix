package jq

import (
	"io"
	"regexp"
	"sync"
)

// Query is a parsed filter. It is safe for concurrent use.
type Query struct {
	root node

	mu      sync.Mutex
	regexes map[string]*regexp.Regexp
}

// Parse compiles a filter. Syntax errors are *SyntaxError values;
// constructs outside the subset are *UnsupportedError values, whose
// message says "<construct> is (or are) not supported in clix --jq". A
// test() pattern written as a string literal is checked here too.
func Parse(expr string) (*Query, error) {
	root, err := parse(expr)
	if err != nil {
		return nil, err
	}
	return &Query{root: root}, nil
}

// Write runs q on input (the text of one JSON value) and writes every
// result to w followed by a newline: a string result raw (its text, as
// jq -r prints it), any other result as jq 1.7 prints it by default
// (indented by two spaces). Results produced before a runtime error are
// written; then the error is returned. Runtime errors are *RuntimeError
// values with messages like jq's (`Cannot index string with string "a"`),
// except that a construct outside the subset found only while running (a
// regex the filter computes, regex flags, a slice) is an *UnsupportedError,
// which "?" does not catch. A malformed input is a plain error.
func (q *Query) Write(w io.Writer, input []byte) error {
	v, err := decode(input)
	if err != nil {
		return err
	}
	return q.run(v, func(out any) error {
		var b []byte
		if s, ok := out.(string); ok {
			b = append([]byte(s), '\n')
		} else {
			b = append(appendJSON(nil, out, true, 0), '\n')
		}
		_, err := w.Write(b)
		return err
	})
}

// run evaluates q on v and passes each result to emit.
func (q *Query) run(v any, emit emitFunc) error {
	ev := &evaluator{q: q}
	return ev.eval(q.root, v, emit)
}

// regex compiles a pattern that was not a literal in the filter, caching
// the result.
func (q *Query) regex(pattern string) (*regexp.Regexp, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if re, ok := q.regexes[pattern]; ok {
		return re, nil
	}
	re, err := compileRegex(pattern)
	if err != nil {
		return nil, err
	}
	if q.regexes == nil {
		q.regexes = make(map[string]*regexp.Regexp)
	}
	if len(q.regexes) < 1000 {
		q.regexes[pattern] = re
	}
	return re, nil
}
