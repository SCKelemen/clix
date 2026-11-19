package filter

import (
	"math/big"
	"strings"
	"time"
	"unicode"

	"github.com/SCKelemen/clix/v2/internal/resource"
)

// Select returns the indices of the resources the filter selects, in
// order. The resources are evaluated in order with one evaluation state,
// as gcloud filters a list: a term whose first non-empty value looks like
// a date and time (YYYY-MM-DD HH:MM:SS, with T or a space) compares dates
// and times for the rest of the list (see the package documentation).
//
// Errors are *resource.UnsupportedError values for what gcloud would do
// that clix does not reproduce, or errors where gcloud fails too.
func (f *Filter) Select(resources []any) ([]int, error) {
	ev := &evaluator{f: f, states: map[*termNode]*termState{}}
	var out []int
	for i, r := range resources {
		ok, err := ev.eval(f.root, r)
		if err != nil {
			return nil, unsupportedIn(err)
		}
		if ok {
			out = append(out, i)
		}
	}
	return out, nil
}

type evaluator struct {
	f      *Filter
	states map[*termNode]*termState
}

// termState is the normalization a term chose (gcloud's
// _ExprOperator._normalize), decided by its first non-empty value.
type termState struct {
	mode  mode
	times []time.Time // the operands as dates and times, in dateTimeMode
}

type mode int

const (
	undecided mode = iota
	identityMode
	dateTimeMode
)

// eval evaluates n for r, with left-to-right short circuits, as gcloud's
// _ExprAND, _ExprOR and _ExprNOT do (so terms see the same values in the
// same order).
func (ev *evaluator) eval(n node, r any) (bool, error) {
	switch n := n.(type) {
	case nil:
		return true, nil
	case andNode:
		l, err := ev.eval(n.left, r)
		if err != nil || !l {
			return false, err
		}
		return ev.eval(n.right, r)
	case orNode:
		l, err := ev.eval(n.left, r)
		if err != nil || l {
			return l, err
		}
		return ev.eval(n.right, r)
	case notNode:
		v, err := ev.eval(n.expr, r)
		return !v, err
	case *termNode:
		return ev.term(n, r)
	}
	return false, nil
}

// term is _ExprOperator.Evaluate: the term is true when any value (each
// element of a non-empty list value) matches any operand.
func (ev *evaluator) term(t *termNode, r any) (bool, error) {
	value, err := resource.Get(r, t.key)
	if err != nil {
		return false, err
	}
	var values []any
	if l, ok := value.([]any); ok && len(l) > 0 {
		values = append(values, l...)
	} else {
		values = []any{value}
	}
	st := ev.states[t]
	if st == nil {
		st = &termState{}
		ev.states[t] = st
	}
	for i, v := range values {
		if resource.Truthy(v) {
			if values[i], err = ev.normalize(t, st, v); err != nil {
				return false, err
			}
		}
	}
	for _, v := range values {
		for j, opd := range t.operands {
			if opd.num != nil {
				pv, n, ok, err := numericType(v)
				if err != nil {
					return false, err
				}
				if ok {
					match, err := t.applyNumber(pv, n, opd)
					if err != nil || match {
						return match, err
					}
					if !opd.constant {
						continue
					}
				}
			}
			if st.mode == dateTimeMode {
				tv, ok := v.(time.Time)
				if !ok {
					continue // an empty value: gcloud skips it
				}
				match, err := t.applyTime(tv, st, j)
				if err != nil || match {
					return match, err
				}
				continue
			}
			match, err := t.applyString(v, opd)
			if err != nil || match {
				return match, err
			}
		}
	}
	return false, nil
}

// numericType is resource_expr._NumericType: the Python value (a bool, an
// int or a float) and its number, ok false where Python raises.
func numericType(v any) (any, resource.Number, bool, error) {
	switch v.(type) {
	case bool, *big.Int, float64:
		n, _ := resource.NumberOf(v)
		return v, n, true, nil
	case string:
		n, ok, err := resource.NumericType(v)
		if err != nil || !ok {
			return nil, n, false, err
		}
		if n.Int != nil {
			return n.Int, n, true, nil
		}
		return n.Float, n, true, nil
	}
	return nil, resource.Number{}, false, nil
}

// applyNumber is Apply for a value read as a number and an operand that
// is a number.
func (t *termNode) applyNumber(pv any, n resource.Number, opd *operand) (bool, error) {
	if t.op.wordMatch() {
		m, err := t.wordMatch(pv)
		if t.op == opNE {
			return !m, err
		}
		return m, err
	}
	c, ok := n.Compare(*opd.num)
	return ok && compareResult(t.op, c), nil
}

func compareResult(o op, c int) bool {
	switch o {
	case opLT:
		return c < 0
	case opLE:
		return c <= 0
	case opGT:
		return c > 0
	case opGE:
		return c >= 0
	}
	return false
}

// applyString is Apply for the value and the operand's text, with
// gcloud's fallbacks for Python's TypeError: a number or bool is compared
// as its repr, and None as the empty string.
func (t *termNode) applyString(v any, opd *operand) (bool, error) {
	if t.op.wordMatch() {
		m, err := t.wordMatch(v)
		if t.op == opNE {
			return !m, err
		}
		return m, err
	}
	switch v := v.(type) {
	case string:
		return compareResult(t.op, strings.Compare(v, opd.text)), nil
	case nil:
		return compareResult(t.op, strings.Compare("", opd.text)), nil
	case bool, *big.Int, float64:
		return compareResult(t.op, strings.Compare(resource.Repr(v), opd.text)), nil
	}
	return false, nil // a list or dict: Python raises TypeError
}

// applyTime compares a date and time with operand j in dateTimeMode.
func (t *termNode) applyTime(v time.Time, st *termState, j int) (bool, error) {
	switch t.op {
	case opHas:
		return false, unsupported("the : operator on dates and times")
	case opEQ, opNE:
		// _WordMatch tries every operand's pattern.
		eq := false
		for _, o := range st.times {
			eq = eq || v.Equal(o)
		}
		if t.op == opNE {
			return !eq, nil
		}
		return eq, nil
	}
	return compareResult(t.op, v.Compare(st.times[j])), nil
}

// wordMatch is _WordMatch: a dict matches when one of its keys or values
// does, a list when one of its elements does, anything else when it
// matches one of the term's patterns.
func (t *termNode) wordMatch(v any) (bool, error) {
	var values []any
	switch v := v.(type) {
	case *resource.Object:
		for _, k := range v.Keys() {
			values = append(values, k)
		}
		for _, k := range v.Keys() {
			x, _ := v.Get(k)
			values = append(values, x)
		}
	case []any:
		values = v
	default:
		values = []any{v}
	}
	for _, x := range values {
		for _, p := range t.patterns {
			m, err := t.matchOne(x, p)
			if err != nil || m {
				return m, err
			}
		}
	}
	return false, nil
}

// matchOne is _MatchOneWordInText: equal numbers, the constants true and
// false for 1 and 0, an equal string, or the pattern matching the text of
// the value (normalized for search: lower case, without accents or HTML
// tags).
func (t *termNode) matchOne(v any, p pattern) (bool, error) {
	var text string
	switch x := v.(type) {
	case bool, *big.Int, float64:
		n, _ := resource.NumberOf(x)
		pn, ok, err := resource.NumericType(p.text)
		if err != nil {
			return false, err
		}
		if ok {
			if c, ok := n.Compare(pn); ok && c == 0 {
				return true, nil
			}
		}
		lower := strings.ToLower(p.text)
		if c, ok := n.Compare(zero); ok && c == 0 && lower == "false" {
			return true, nil
		}
		if c, ok := n.Compare(one); ok && c == 0 && lower == "true" {
			return true, nil
		}
		text = trimPointZeros(resource.Repr(x))
	case string:
		if x == p.text {
			return true, nil
		}
		text = resource.NormalizeForSearch(x, true)
	case nil:
		if p.text == "" {
			return true, nil
		}
		if p.text == "*" && t.op == opHas {
			return false, nil
		}
		text = "null"
	default:
		text = resource.NormalizeForSearch(x, true)
	}
	if p.any {
		return strings.ContainsFunc(text, func(r rune) bool { return r != '\n' }), nil
	}
	if p.word {
		return strings.ContainsFunc(text, isWordChar), nil
	}
	if p.re.MatchString(text) {
		return true, nil
	}
	if t.zone {
		if i := strings.LastIndexByte(text, '/'); i >= 0 {
			return p.re.MatchString(text[i+1:]), nil
		}
	}
	return false, nil
}

// isWordChar is Python's \w for str patterns: a letter, a number or an
// underscore.
func isWordChar(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r)
}

var (
	zero = resource.Number{Int: big.NewInt(0)}
	one  = resource.Number{Int: big.NewInt(1)}
)

// trimPointZeros is re.sub(r'\.0*$', ”, s): "10.0" becomes "10".
func trimPointZeros(s string) string {
	t := strings.TrimRight(s, "0")
	if strings.HasSuffix(t, ".") {
		return t[:len(t)-1]
	}
	return s
}

// normalize is the term's normalization of a non-empty value; the first
// one decides it (_ExprOperator.InitializeNormalization).
func (ev *evaluator) normalize(t *termNode, st *termState, v any) (any, error) {
	switch st.mode {
	case identityMode:
		return v, nil
	case dateTimeMode:
		s, ok := v.(string)
		if !ok {
			return nil, unsupported("comparing %s, which holds dates and times, when a value (%s) is not a string,", t.key, resource.Repr(v))
		}
		tv, err := parseDateTime(s, ev.f.loc)
		if err != nil {
			return nil, unsupported("reading %q of %s, which holds dates and times, as a date and time (%v)", s, t.key, err)
		}
		return tv, nil
	}
	st.mode = identityMode
	s, ok := v.(string)
	if !ok || !looksLikeDateTime(s) {
		return v, nil
	}
	tv, err := parseDateTime(s, ev.f.loc)
	if err != nil {
		return nil, unsupported("reading %q of %s as a date and time (%v)", s, t.key, err)
	}
	if t.op == opHas {
		return nil, unsupported("the : operator on %s, which holds dates and times,", t.key)
	}
	if t.list {
		// gcloud fails reading a list of operands as dates and times.
		return nil, unsupported("a list of operands for %s, which holds dates and times,", t.key)
	}
	times := make([]time.Time, len(t.operands))
	for i, o := range t.operands {
		if times[i], err = parseDateTime(o.text, ev.f.loc); err != nil {
			return nil, unsupported("comparing %s, which holds dates and times, with %q (%v)", t.key, o.text, err)
		}
	}
	st.mode, st.times = dateTimeMode, times
	return tv, nil
}

// looksLikeDateTime is gcloud's re.match(r'\d\d\d\d-\d\d-\d\d[ T]\d\d:\d\d:\d\d')
// (Python's \d matches any decimal digit).
func looksLikeDateTime(s string) bool {
	const shape = "dddd-dd-dd_dd:dd:dd"
	r := []rune(s)
	if len(r) < len(shape) {
		return false
	}
	for i, c := range shape {
		switch c {
		case 'd':
			if !unicode.IsDigit(r[i]) {
				return false
			}
		case '_':
			if r[i] != ' ' && r[i] != 'T' {
				return false
			}
		default:
			if r[i] != c {
				return false
			}
		}
	}
	return true
}
