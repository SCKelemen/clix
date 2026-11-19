package jq

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

// emitFunc receives each output of a filter. Returning an error stops the
// evaluation.
type emitFunc func(any) error

// RuntimeError is an error raised while running a filter, such as
// indexing a string with a key.
type RuntimeError struct{ Msg string }

func (e *RuntimeError) Error() string { return e.Msg }

func runtimeErrorf(format string, args ...any) error {
	return &RuntimeError{Msg: fmt.Sprintf(format, args...)}
}

// unsupportedAtRuntime reports a construct outside the subset that only
// shows up while running (such as a regex computed by the filter). It is
// an *UnsupportedError, not a *RuntimeError, so neither "?" nor an
// optional index can hide it: the filter fails instead of giving results
// that differ from jq's.
func unsupportedAtRuntime(construct string) error {
	return &UnsupportedError{Offset: -1, Construct: construct}
}

// passThrough carries an error raised after a try's body produced a
// value (by the rest of the filter), so that the try does not catch it.
type passThrough struct {
	err   error
	owner *tryNode
}

func (p *passThrough) Error() string { return p.err.Error() }

// stop ends a generator early (first(f)).
type stop struct{}

func (*stop) Error() string { return "stop" }

type evaluator struct {
	q *Query
}

func (ev *evaluator) eval(n node, in any, emit emitFunc) error {
	switch n := n.(type) {
	case identityNode:
		return emit(in)
	case recurseNode:
		return ev.recurse(in, emit)
	case literalNode:
		return emit(n.v)
	case negatedNode:
		return emit(n.v)
	case indexNode:
		return ev.eval(n.key, in, func(k any) error {
			return ev.eval(n.target, in, func(t any) error {
				v, err := index(t, k)
				if err != nil {
					if n.opt && isRuntime(err) {
						return nil
					}
					return err
				}
				return emit(v)
			})
		})
	case iterateNode:
		return ev.eval(n.target, in, func(t any) error {
			if n.opt {
				switch t.(type) {
				case []any, *object:
				default:
					return nil
				}
			}
			return iterate(t, emit)
		})
	case tryNode:
		owner := &n
		err := ev.eval(n.body, in, func(v any) error {
			if err := emit(v); err != nil {
				return &passThrough{err: err, owner: owner}
			}
			return nil
		})
		var p *passThrough
		if errors.As(err, &p) && p.owner == owner {
			return p.err
		}
		if err != nil && !isRuntime(err) {
			return err
		}
		return nil
	case pipeNode:
		return ev.eval(n.left, in, func(v any) error {
			return ev.eval(n.right, v, emit)
		})
	case commaNode:
		if err := ev.eval(n.left, in, emit); err != nil {
			return err
		}
		return ev.eval(n.right, in, emit)
	case altNode:
		found := false
		err := ev.eval(n.left, in, func(v any) error {
			if !truthy(v) {
				return nil
			}
			found = true
			return emit(v)
		})
		if err != nil || found {
			return err
		}
		return ev.eval(n.right, in, emit)
	case andNode:
		return ev.eval(n.left, in, func(l any) error {
			if !truthy(l) {
				return emit(false)
			}
			return ev.eval(n.right, in, func(r any) error { return emit(truthy(r)) })
		})
	case orNode:
		return ev.eval(n.left, in, func(l any) error {
			if truthy(l) {
				return emit(true)
			}
			return ev.eval(n.right, in, func(r any) error { return emit(truthy(r)) })
		})
	case cmpNode:
		// jq evaluates the right operand in the outer loop.
		return ev.eval(n.right, in, func(r any) error {
			return ev.eval(n.left, in, func(l any) error {
				return emit(compareOp(n.op, l, r))
			})
		})
	case arrayNode:
		arr := []any{}
		if n.body != nil {
			err := ev.eval(n.body, in, func(v any) error {
				arr = append(arr, v)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return emit(arr)
	case objectNode:
		return ev.object(n.entries, in, newObject(0), emit)
	case stringNode:
		return ev.interpolate(n.parts, in, emit)
	case ifNode:
		return ev.eval(n.cond, in, func(c any) error {
			switch {
			case truthy(c):
				return ev.eval(n.then, in, emit)
			case n.els != nil:
				return ev.eval(n.els, in, emit)
			}
			return emit(in)
		})
	case callNode:
		return ev.call(n, in, emit)
	case formatNode:
		s, err := format(n.name, in)
		if err != nil {
			return err
		}
		return emit(s)
	}
	panic(fmt.Sprintf("jq: unknown node %T", n))
}

// isRuntime reports whether err was raised by the filter (and so may be
// caught by "?"), rather than by the writer receiving the results.
func isRuntime(err error) bool {
	var r *RuntimeError
	return errors.As(err, &r)
}

// recurse is "..": the input, then everything inside it, depth first.
func (ev *evaluator) recurse(in any, emit emitFunc) error {
	if err := emit(in); err != nil {
		return err
	}
	switch x := in.(type) {
	case []any:
		for _, e := range x {
			if err := ev.recurse(e, emit); err != nil {
				return err
			}
		}
	case *object:
		for _, k := range x.keys {
			if err := ev.recurse(x.vals[k], emit); err != nil {
				return err
			}
		}
	}
	return nil
}

// object builds the objects of {...}: every combination of the entries'
// keys and values, the first entry varying slowest.
func (ev *evaluator) object(entries []objectEntry, in any, acc *object, emit emitFunc) error {
	if len(entries) == 0 {
		return emit(acc)
	}
	e := entries[0]
	return ev.eval(e.key, in, func(k any) error {
		insert := func(v any) error {
			// As in jq, the key's type is checked when the pair is
			// inserted, after its value was produced.
			key, ok := k.(string)
			if !ok {
				return runtimeErrorf("Cannot use %s as object key", describe(k))
			}
			return ev.object(entries[1:], in, acc.with(key, v), emit)
		}
		if e.shorthand {
			v, err := index(in, k)
			if err != nil {
				return err
			}
			return insert(v)
		}
		return ev.eval(e.value, in, insert)
	})
}

// interpolate builds the strings of "...\(e)...". As in jq, where the
// string is a chain of "+", the last interpolation varies slowest.
func (ev *evaluator) interpolate(parts []strPart, in any, emit emitFunc) error {
	if len(parts) == 0 {
		return emit("")
	}
	last := parts[len(parts)-1]
	rest := parts[:len(parts)-1]
	if last.expr == nil {
		return ev.interpolate(rest, in, func(prefix any) error {
			return emit(prefix.(string) + last.text)
		})
	}
	return ev.eval(last.expr, in, func(v any) error {
		s := toString(v)
		return ev.interpolate(rest, in, func(prefix any) error {
			return emit(prefix.(string) + s)
		})
	})
}

func compareOp(op string, l, r any) bool {
	c := compare(l, r)
	switch op {
	case "==":
		return c == 0
	case "!=":
		return c != 0
	case "<":
		return c < 0
	case "<=":
		return c <= 0
	case ">":
		return c > 0
	case ">=":
		return c >= 0
	}
	panic("jq: unknown comparison " + op)
}

// index is t[k] (jq's jv_get).
func index(t, k any) (any, error) {
	switch x := t.(type) {
	case *object:
		if key, ok := k.(string); ok {
			return x.vals[key], nil
		}
	case []any:
		switch key := k.(type) {
		case number:
			return arrayIndex(x, key.f), nil
		case []any:
			return indexes(x, key), nil
		case *object:
			return nil, unsupportedAtRuntime("slices (.[a:b])")
		}
	case nil:
		switch k.(type) {
		case string, number, *object:
			return nil, nil
		}
	case string:
		if _, ok := k.(*object); ok {
			return nil, unsupportedAtRuntime("slices (.[a:b])")
		}
	}
	if key, ok := k.(string); ok {
		return nil, runtimeErrorf("Cannot index %s with string %s", typeName(t), compactString(key))
	}
	return nil, runtimeErrorf("Cannot index %s with %s", typeName(t), typeName(k))
}

// indexes is arr[sub] for an array sub: the positions where sub occurs in
// arr, as jq's jv_array_indexes computes them.
func indexes(arr, sub []any) []any {
	out := []any{}
	if len(sub) == 0 {
		return out
	}
	for i := range arr {
		match := true
		for j, y := range sub {
			if i+j >= len(arr) || compare(arr[i+j], y) != 0 {
				match = false
				break
			}
		}
		if match {
			out = append(out, computed(float64(i)))
		}
	}
	return out
}

// arrayIndex returns arr[i] for a numeric index as jq does: the index is
// truncated toward zero, a negative one counts from the end, and an index
// out of range (or NaN) gives null.
func arrayIndex(arr []any, f float64) any {
	i, ok := intIndex(f)
	if !ok {
		return nil
	}
	if i < 0 {
		i += len(arr)
	}
	if i < 0 || i >= len(arr) {
		return nil
	}
	return arr[i]
}

// intIndex truncates f to an int index, as C's (int) cast does; ok is
// false for NaN and values outside the 32-bit range jq uses.
func intIndex(f float64) (int, bool) {
	if math.IsNaN(f) {
		return 0, false
	}
	t := math.Trunc(f)
	if t < math.MinInt32 || t > math.MaxInt32 {
		return 0, false
	}
	return int(t), true
}

// iterate is t[].
func iterate(t any, emit emitFunc) error {
	switch x := t.(type) {
	case []any:
		for _, e := range x {
			if err := emit(e); err != nil {
				return err
			}
		}
		return nil
	case *object:
		for _, k := range x.keys {
			if err := emit(x.vals[k]); err != nil {
				return err
			}
		}
		return nil
	}
	return runtimeErrorf("Cannot iterate over %s", describe(t))
}

// keysOf is jq's keys (sorted) and keys_unsorted.
func keysOf(v any, sorted bool) ([]any, error) {
	switch x := v.(type) {
	case *object:
		keys := x.keys
		if sorted {
			keys = x.sortedKeys()
		}
		out := make([]any, len(keys))
		for i, k := range keys {
			out[i] = k
		}
		return out, nil
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = computed(float64(i))
		}
		return out, nil
	}
	return nil, runtimeErrorf("%s has no keys", describe(v))
}

func length(v any) (any, error) {
	switch x := v.(type) {
	case nil:
		return computed(0), nil
	case number:
		return x.abs(), nil
	case string:
		n := 0
		for range x {
			n++
		}
		return computed(float64(n)), nil
	case []any:
		return computed(float64(len(x))), nil
	case *object:
		return computed(float64(len(x.keys))), nil
	}
	return nil, runtimeErrorf("%s has no length", describe(v))
}

func has(v, k any) (any, error) {
	switch x := v.(type) {
	case *object:
		if key, ok := k.(string); ok {
			_, found := x.vals[key]
			return found, nil
		}
	case []any:
		if n, ok := k.(number); ok {
			i, ok := intIndex(n.f)
			return ok && i >= 0 && i < len(x), nil
		}
	case nil:
		// jq: null has no keys.
		return false, nil
	}
	return nil, runtimeErrorf("Cannot check whether %s has a %s key", typeName(v), typeName(k))
}

func toEntries(v any) (any, error) {
	keys, err := keysOf(v, false)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(keys))
	for i, k := range keys {
		value, err := index(v, k)
		if err != nil {
			return nil, err
		}
		e := newObject(2)
		e.set("key", k)
		e.set("value", value)
		out[i] = e
	}
	return out, nil
}

// asciiCase is ascii_downcase and ascii_upcase.
func asciiCase(v any, upper bool) (any, error) {
	s, ok := v.(string)
	if !ok {
		return nil, runtimeErrorf("explode input must be a string")
	}
	b := []byte(s)
	for i, c := range b {
		switch {
		case upper && c >= 'a' && c <= 'z':
			b[i] = c - 32
		case !upper && c >= 'A' && c <= 'Z':
			b[i] = c + 32
		}
	}
	return string(b), nil
}

// join is jq 1.8.1's definition (src/builtin.jq):
//
//	def join($x): reduce .[] as $i (null;
//	    (if .==null then "" else .+$x end) +
//	    ($i | if type=="boolean" or type=="number" then tostring else .//"" end)
//	) // "";
func join(v, sep any) (any, error) {
	var acc any // null
	err := iterate(v, func(item any) error {
		var prefix any = ""
		if acc != nil {
			p, err := add(acc, sep)
			if err != nil {
				return err
			}
			prefix = p
		}
		var piece any
		switch item.(type) {
		case nil:
			piece = ""
		case bool, number:
			piece = compactString(item)
		default:
			piece = item
		}
		s, err := add(prefix, piece)
		if err != nil {
			return err
		}
		acc = s
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !truthy(acc) {
		return "", nil
	}
	return acc, nil
}

// add is jq's "+" for the cases join reaches: null on either side gives
// the other operand, two strings concatenate, anything else is an error.
// (Numbers never reach it.)
func add(a, b any) (any, error) {
	switch {
	case a == nil:
		return b, nil
	case b == nil:
		return a, nil
	}
	if x, ok := a.(string); ok {
		if y, ok := b.(string); ok {
			return x + y, nil
		}
	}
	return nil, runtimeErrorf("%s and %s cannot be added", describe(a), describe(b))
}

func sortValues(v any) (any, error) {
	arr, ok := v.([]any)
	if !ok {
		return nil, runtimeErrorf("%s cannot be sorted, as it is not an array", describe(v))
	}
	out := append([]any(nil), arr...)
	sort.SliceStable(out, func(i, j int) bool { return compare(out[i], out[j]) < 0 })
	return out, nil
}

// format applies @text, @json, @csv or @tsv.
func format(name string, v any) (any, error) {
	switch name {
	case "text":
		return toString(v), nil
	case "json":
		return compactString(v), nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, runtimeErrorf("%s cannot be %s-formatted, only array", describe(v), name)
	}
	var b strings.Builder
	for i, e := range arr {
		if i > 0 {
			if name == "csv" {
				b.WriteByte(',')
			} else {
				b.WriteByte('\t')
			}
		}
		switch x := e.(type) {
		case nil:
		case bool, number:
			b.WriteString(compactString(x))
		case string:
			if name == "csv" {
				b.WriteByte('"')
				b.WriteString(csvEscaper.Replace(x))
				b.WriteByte('"')
			} else {
				b.WriteString(tsvEscaper.Replace(x))
			}
		default:
			return nil, runtimeErrorf("%s is not valid in a csv row", describe(e))
		}
	}
	return b.String(), nil
}

// jq escapes NUL as \0 in every format, besides the format's own escapes.
var (
	csvEscaper = strings.NewReplacer(`"`, `""`, "\x00", `\0`)
	tsvEscaper = strings.NewReplacer(`\`, `\\`, "\t", `\t`, "\r", `\r`, "\n", `\n`, "\x00", `\0`)
)

func (ev *evaluator) call(c callNode, in any, emit emitFunc) error {
	unary := func(f func(any) (any, error)) error {
		v, err := f(in)
		if err != nil {
			return err
		}
		return emit(v)
	}
	switch c.name {
	case "empty":
		return nil
	case "not":
		return emit(!truthy(in))
	case "length":
		return unary(length)
	case "keys", "keys_unsorted":
		keys, err := keysOf(in, c.name == "keys")
		if err != nil {
			return err
		}
		return emit(keys)
	case "to_entries":
		return unary(toEntries)
	case "tostring":
		return emit(toString(in))
	case "tojson":
		return emit(compactString(in))
	case "ascii_downcase", "ascii_upcase":
		v, err := asciiCase(in, c.name == "ascii_upcase")
		if err != nil {
			return err
		}
		return emit(v)
	case "type":
		return emit(typeName(in))
	case "values":
		if in == nil {
			return nil
		}
		return emit(in)
	case "sort":
		return unary(sortValues)
	case "first", "last":
		if len(c.args) == 0 {
			i := 0.0
			if c.name == "last" {
				i = -1
			}
			v, err := index(in, computed(i))
			if err != nil {
				return err
			}
			return emit(v)
		}
		if c.name == "last" {
			// As in jq 1.8, last(empty) gives no output, like first(empty).
			var last any
			found := false
			err := ev.eval(c.args[0], in, func(v any) error {
				last, found = v, true
				return nil
			})
			if err != nil || !found {
				return err
			}
			return emit(last)
		}
		done := &stop{}
		var first any
		err := ev.eval(c.args[0], in, func(v any) error {
			first = v
			return done
		})
		if err == nil {
			return nil
		}
		if err != done {
			return err
		}
		return emit(first)
	case "select":
		return ev.eval(c.args[0], in, func(v any) error {
			if truthy(v) {
				return emit(in)
			}
			return nil
		})
	case "map":
		out := []any{}
		err := iterate(in, func(item any) error {
			return ev.eval(c.args[0], item, func(v any) error {
				out = append(out, v)
				return nil
			})
		})
		if err != nil {
			return err
		}
		return emit(out)
	case "has":
		return ev.eval(c.args[0], in, func(k any) error {
			v, err := has(in, k)
			if err != nil {
				return err
			}
			return emit(v)
		})
	case "join":
		return ev.eval(c.args[0], in, func(sep any) error {
			v, err := join(in, sep)
			if err != nil {
				return err
			}
			return emit(v)
		})
	case "test":
		return ev.eval(c.args[0], in, func(re any) error {
			v, err := ev.test(c, in, re)
			if err != nil {
				return err
			}
			return emit(v)
		})
	case "sort_by":
		return ev.sortBy(c.args[0], in, emit)
	}
	panic("jq: unknown function " + c.name)
}

// sortBy is jq's def sort_by(f): _sort_by_impl(map([f])).
func (ev *evaluator) sortBy(f node, in any, emit emitFunc) error {
	var keys []any
	err := iterate(in, func(item any) error {
		var key []any
		err := ev.eval(f, item, func(v any) error {
			key = append(key, v)
			return nil
		})
		if key == nil {
			key = []any{}
		}
		keys = append(keys, key)
		return err
	})
	if err != nil {
		return err
	}
	arr, ok := in.([]any)
	if !ok {
		if keys == nil {
			keys = []any{}
		}
		return runtimeErrorf("%s and %s cannot be sorted, as they are not both arrays", describe(in), describe(keys))
	}
	idx := make([]int, len(arr))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(i, j int) bool { return compare(keys[idx[i]], keys[idx[j]]) < 0 })
	out := make([]any, len(arr))
	for i, j := range idx {
		out[i] = arr[j]
	}
	return emit(out)
}

// test is jq's test/1. Its argument may be a pattern, or an array of a
// pattern and flags (jq ignores anything after them); flags other than
// null and "" are not supported.
func (ev *evaluator) test(c callNode, in, re any) (any, error) {
	if arr, ok := re.([]any); ok {
		switch {
		case len(arr) > 1 && arr[1] != nil && arr[1] != "":
			return nil, unsupportedAtRuntime("regex flags (test([re, flags]))")
		case len(arr) > 0:
			re = arr[0]
		}
	}
	pattern, ok := re.(string)
	if !ok {
		return nil, runtimeErrorf("%s not a string or array", typeName(re))
	}
	s, ok := in.(string)
	if !ok {
		return nil, runtimeErrorf("%s cannot be matched, as it is not a string", describe(in))
	}
	compiled := c.re
	if compiled == nil {
		var err error
		compiled, err = ev.q.regex(pattern)
		if err != nil {
			var u *UnsupportedError
			if errors.As(err, &u) {
				return nil, err
			}
			return nil, &RuntimeError{Msg: err.Error()}
		}
	}
	return compiled.MatchString(s), nil
}
