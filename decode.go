package clix

import (
	"bytes"
	"encoding"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// Strict decoding (UnmarshalStrict, ConfigManager.Decode and DecodeVar)
// reads YAML or JSON into Go values in the spirit of Kubernetes'
// sigs.k8s.io/json UnmarshalStrict
// (https://github.com/kubernetes-sigs/json): the document is turned into
// JSON and decoded with encoding/json/v2, which matches names exactly,
// and every unknown field, duplicate key and value of the wrong type is
// reported, each with its path. Problems that encoding/json/v2 reports
// carry a JSON Pointer (RFC 6901,
// https://datatracker.ietf.org/doc/html/rfc6901), which is turned back
// into the path.

// ErrUnknownField is the error of a FieldError for a field that the
// target type does not have.
var ErrUnknownField = errors.New("unknown field")

// ErrDuplicateField is the error of a FieldError for a key written more
// than once in one mapping.
var ErrDuplicateField = errors.New("duplicate field")

// FieldError is one problem that strict decoding found.
type FieldError struct {
	// Path names the field, with dots between names and [i] for list
	// items, as Kubernetes does: "spec.containers[0].image". It is ""
	// for the document itself.
	Path string

	// Err is ErrUnknownField, ErrDuplicateField, or why the value does
	// not fit the field, such as "expected int, got a string".
	Err error
}

// Error returns `unknown field "spec.x"`, `duplicate field "spec.x"` or
// "spec.x: <why>".
func (e *FieldError) Error() string {
	switch {
	case errors.Is(e.Err, ErrUnknownField):
		return fmt.Sprintf("unknown field %q", e.Path)
	case errors.Is(e.Err, ErrDuplicateField):
		return fmt.Sprintf("duplicate field %q", e.Path)
	case e.Path == "":
		return e.Err.Error()
	}
	return e.Path + ": " + e.Err.Error()
}

// Unwrap returns Err.
func (e *FieldError) Unwrap() error { return e.Err }

// StrictError lists every problem strict decoding found, ordered by path:
// unknown fields, duplicate keys and values that do not fit their
// fields. When it is returned, the target was not changed. errors.Is
// reports whether it holds ErrUnknownField or ErrDuplicateField.
type StrictError struct {
	// Source names what was decoded, such as a file name, or is empty.
	Source string

	// Fields holds the problems.
	Fields []*FieldError
}

// Error returns a message such as
// `spec.yaml: strict decoding error: unknown field "spec.replcas", duplicate field "spec.name"`.
func (e *StrictError) Error() string {
	var b strings.Builder
	if e.Source != "" {
		b.WriteString(e.Source)
		b.WriteString(": ")
	}
	b.WriteString("strict decoding error: ")
	for i, f := range e.Fields {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(f.Error())
	}
	return b.String()
}

// Unwrap returns the FieldErrors.
func (e *StrictError) Unwrap() []error {
	errs := make([]error, len(e.Fields))
	for i, f := range e.Fields {
		errs[i] = f
	}
	return errs
}

// UnmarshalStrict decodes data, one YAML or JSON document, into v, which
// must be a non-nil pointer, strictly:
//
//   - a field the target type does not have is an error
//     (ErrUnknownField);
//   - a key written twice in one mapping is an error (ErrDuplicateField),
//     even where the target would ignore it;
//   - names match exactly, case included: struct fields match their json
//     tag name, or their Go name when untagged (encoding/json/v2's rules,
//     with embedded structs inlined);
//   - a value of the wrong type, such as a string for an int field, is an
//     error.
//
// Every problem is reported, not just the first: the error is a
// *StrictError whose FieldErrors name each field by its path
// ("spec.containers[0].image"). When it is returned, v is unchanged;
// otherwise v is updated as encoding/json/v2 does, so fields the document
// leaves out keep their values (defaults set before the call survive),
// nested structs and maps are merged, and lists are replaced.
//
// Scalars keep the text written in the document where the field is
// text: "version: 1.10" sets a string field to "1.10" and "mode: 0755" to
// "0755", and so does a field whose type implements
// encoding.TextUnmarshaler; a number field gets YAML's number, so 0755 is
// 493 and 0x1F is 31, as int flags read the config file. A quoted string
// is never a number: "port: '80'" into an int field is an error. A
// time.Duration field reads Go's duration text ("90s", "1h30m"). YAML
// anchors, aliases and merge keys (<<) work; a document whose aliases
// expand past a million values is an error. An empty document changes
// nothing.
func UnmarshalStrict(data []byte, v any) error {
	return decodeDocument(data, v, decodeOptions{})
}

// decodeOptions say how a decode reports problems.
type decodeOptions struct {
	source    string // what is decoded, for messages
	base      string // the path of the document's root, such as a config key
	sensitive bool   // leave values, and messages that might hold them, out of errors
}

// maxDecodeNodes is how many values a YAML document may expand to through
// aliases and merge keys before strict decoding gives up.
const maxDecodeNodes = 1 << 20

// maxStrictProblems is how many problems a StrictError lists.
const maxStrictProblems = 64

// errTooManyProblems ends a StrictError that had more problems to list.
var errTooManyProblems = errors.New("too many errors")

// decodeDocument decodes data, a YAML or JSON document, into v.
func decodeDocument(data []byte, v any, o decodeOptions) error {
	if err := checkTarget(v); err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil // empty, or only comments
		}
		return o.wrap(err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return o.wrap(err)
		}
		return o.wrap(fmt.Errorf("line %d: only one YAML document is allowed", extra.Line))
	}
	n := &doc
	if n.Kind == yaml.DocumentNode {
		if len(n.Content) == 0 {
			return nil
		}
		n = n.Content[0]
	}
	c := newYAMLConverter(o.base)
	root, err := c.convert(n, "", nil)
	if err != nil {
		return o.wrap(err)
	}
	return decodeTree(root, c, v, o)
}

// wrap prefixes err with the source.
func (o decodeOptions) wrap(err error) error {
	if o.source == "" {
		return err
	}
	return fmt.Errorf("%s: %w", o.source, err)
}

// checkTarget reports whether v can be decoded into.
func checkTarget(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("clix: strict decoding needs a non-nil pointer, not %T", v)
	}
	return nil
}

// jsonNode is a JSON value built from YAML: an object (with its members
// in order), an array, or a scalar as JSON text.
type jsonNode struct {
	kind  byte // '{', '[' or 's'
	raw   string
	keys  []string
	vals  []*jsonNode
	items []*jsonNode
}

// yamlConverter turns YAML nodes into jsonNodes, recording duplicate keys
// and the text of numbers and booleans as written.
type yamlConverter struct {
	base   string            // the path of the root, for problems
	text   map[string]string // JSON pointer -> YAML text of a number or boolean
	dups   []*FieldError     // keys written twice
	budget int               // values left to convert (see maxDecodeNodes)
}

func newYAMLConverter(base string) *yamlConverter {
	return &yamlConverter{base: base, text: make(map[string]string), budget: maxDecodeNodes}
}

// spend takes n from the budget.
func (c *yamlConverter) spend(n int) error {
	c.budget -= n
	if c.budget < 0 {
		return fmt.Errorf("the document expands to more than %d values (YAML aliases)", maxDecodeNodes)
	}
	return nil
}

// convert turns n, at the JSON pointer ptr and the problem path path, into
// a jsonNode.
func (c *yamlConverter) convert(n *yaml.Node, ptr jsontext.Pointer, path []pathPart) (*jsonNode, error) {
	if err := c.spend(1); err != nil {
		return nil, err
	}
	n = resolveAlias(n)
	switch n.Kind {
	case yaml.ScalarNode:
		return c.scalar(n, ptr), nil
	case yaml.SequenceNode:
		node := &jsonNode{kind: '['}
		for i, item := range n.Content {
			child, err := c.convert(item, ptr.AppendToken(strconv.Itoa(i)), append(path[:len(path):len(path)], pathPart{index: i, isIndex: true}))
			if err != nil {
				return nil, err
			}
			node.items = append(node.items, child)
		}
		return node, nil
	case yaml.MappingNode:
		pairs, err := c.pairs(n, 0)
		if err != nil {
			return nil, err
		}
		node := &jsonNode{kind: '{'}
		seen := make(map[string]bool, len(pairs))
		for _, p := range pairs {
			childPath := append(path[:len(path):len(path)], pathPart{name: p.key})
			if seen[p.key] {
				if !p.merged {
					c.dups = append(c.dups, &FieldError{Path: c.pathString(childPath), Err: ErrDuplicateField})
				}
				continue
			}
			seen[p.key] = true
			child, err := c.convert(p.value, ptr.AppendToken(p.key), childPath)
			if err != nil {
				return nil, err
			}
			node.keys = append(node.keys, p.key)
			node.vals = append(node.vals, child)
		}
		return node, nil
	}
	return nil, fmt.Errorf("line %d: unsupported YAML node", n.Line)
}

// pairs returns the pairs of mapping n with merge keys applied, as
// mappingPairs does, but spending the budget for every pair, so merge
// keys that name each other many times cannot run without end. Unlike
// mappingPairs it keeps a key written twice, for convert to report.
func (c *yamlConverter) pairs(n *yaml.Node, depth int) ([]yamlPair, error) {
	if depth > 100 {
		return nil, fmt.Errorf("line %d: merge keys nested too deeply", n.Line)
	}
	if err := c.spend(len(n.Content) / 2); err != nil {
		return nil, err
	}
	var pairs []yamlPair
	var sources []*yaml.Node
	own := make(map[string]bool)
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, value := resolveAlias(n.Content[i]), n.Content[i+1]
		if key.ShortTag() == "!!merge" {
			v := resolveAlias(value)
			if v.Kind == yaml.SequenceNode {
				for _, item := range v.Content {
					sources = append(sources, resolveAlias(item))
				}
			} else {
				sources = append(sources, v)
			}
			continue
		}
		if key.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("line %d: keys must be scalars", key.Line)
		}
		pairs = append(pairs, yamlPair{key: key.Value, value: value})
		own[key.Value] = true
	}
	for _, src := range sources {
		if src.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("line %d: a merge key (<<) needs a mapping", src.Line)
		}
		merged, err := c.pairs(src, depth+1)
		if err != nil {
			return nil, err
		}
		for _, p := range merged {
			if own[p.key] {
				continue
			}
			own[p.key] = true
			pairs = append(pairs, yamlPair{key: p.key, value: p.value, merged: true})
		}
	}
	return pairs, nil
}

// scalar turns the YAML scalar n at ptr into JSON: null, a boolean, a
// number, or a string with the text as written. A number is written as
// JSON when its text is a JSON number ("1.10" stays 1.10), and otherwise
// as YAML's decimal reading ("0755" is 493); a float JSON cannot hold
// (.inf, .nan) is a string. The text of numbers and booleans is recorded
// for fields that take text (see textUnmarshaler).
func (c *yamlConverter) scalar(n *yaml.Node, ptr jsontext.Pointer) *jsonNode {
	switch n.ShortTag() {
	case "!!null":
		return &jsonNode{kind: 's', raw: "null"}
	case "!!bool":
		var b bool
		if n.Decode(&b) == nil {
			c.text[string(ptr)] = n.Value
			return &jsonNode{kind: 's', raw: strconv.FormatBool(b)}
		}
	case "!!int", "!!float":
		raw := n.Value
		if !isJSONNumber(raw) {
			raw = yamlNumber(n)
		}
		if isJSONNumber(raw) {
			c.text[string(ptr)] = n.Value
			return &jsonNode{kind: 's', raw: raw}
		}
	}
	return &jsonNode{kind: 's', raw: quoteJSON(n.Value)}
}

// isJSONNumber reports whether s is a JSON number.
func isJSONNumber(s string) bool {
	if s == "" || (s[0] != '-' && (s[0] < '0' || s[0] > '9')) {
		return false
	}
	return jsontext.Value(s).IsValid()
}

// quoteJSON returns s as a JSON string.
func quoteJSON(s string) string {
	b, _ := jsontext.AppendQuote(nil, s) // YAML text is valid UTF-8
	return string(b)
}

// pathPart is one step of a problem path: a name, or a list index.
type pathPart struct {
	name    string
	index   int
	isIndex bool
}

// pathString formats path as FieldError.Path does, after the converter's
// base.
func (c *yamlConverter) pathString(path []pathPart) string {
	var b strings.Builder
	b.WriteString(c.base)
	for _, p := range path {
		if p.isIndex {
			fmt.Fprintf(&b, "[%d]", p.index)
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('.')
		}
		b.WriteString(p.name)
	}
	return b.String()
}

// encode writes n as JSON.
func (n *jsonNode) encode(b *bytes.Buffer) {
	switch n.kind {
	case '{':
		b.WriteByte('{')
		for i, k := range n.keys {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(quoteJSON(k))
			b.WriteByte(':')
			n.vals[i].encode(b)
		}
		b.WriteByte('}')
	case '[':
		b.WriteByte('[')
		for i, item := range n.items {
			if i > 0 {
				b.WriteByte(',')
			}
			item.encode(b)
		}
		b.WriteByte(']')
	default:
		b.WriteString(n.raw)
	}
}

// walk follows ptr from n. It returns the node ptr refers to, its parent
// and the path parts on the way; parent is nil for the root. ok is false
// when ptr leaves the tree.
func (n *jsonNode) walk(ptr jsontext.Pointer) (node, parent *jsonNode, path []pathPart, ok bool) {
	node = n
	for tok := range ptr.Tokens() {
		parent = node
		switch node.kind {
		case '{':
			i := indexOf(node.keys, tok)
			if i < 0 {
				return nil, nil, path, false
			}
			path = append(path, pathPart{name: tok})
			node = node.vals[i]
		case '[':
			i, err := strconv.Atoi(tok)
			if err != nil || i < 0 || i >= len(node.items) {
				return nil, nil, path, false
			}
			path = append(path, pathPart{index: i, isIndex: true})
			node = node.items[i]
		default:
			return nil, nil, path, false
		}
	}
	return node, parent, path, true
}

func indexOf(keys []string, key string) int {
	for i, k := range keys {
		if k == key {
			return i
		}
	}
	return -1
}

// remove deletes the member ptr refers to from its object, or replaces
// any other value with null, which every Go type accepts, so decoding
// can go on to find the next problem. It reports whether the tree
// changed.
func (n *jsonNode) remove(ptr jsontext.Pointer) bool {
	node, parent, _, ok := n.walk(ptr)
	if !ok {
		return false
	}
	if parent != nil && parent.kind == '{' {
		i := indexOf(parent.keys, ptr.LastToken())
		parent.keys = append(parent.keys[:i], parent.keys[i+1:]...)
		parent.vals = append(parent.vals[:i], parent.vals[i+1:]...)
		return true
	}
	if node.kind == 's' && node.raw == "null" {
		return false
	}
	*node = jsonNode{kind: 's', raw: "null"}
	return true
}

// decodeTree decodes root into v strictly. It first decodes into fresh
// values of v's type, removing each problem it finds so the next one
// shows, and only when there are none decodes into v itself.
func decodeTree(root *jsonNode, c *yamlConverter, v any, o decodeOptions) error {
	problems := append([]*FieldError(nil), c.dups...)
	opts := strictOptions(c.text)
	typ := reflect.TypeOf(v).Elem()
	var buf bytes.Buffer
	for len(problems) < maxStrictProblems {
		buf.Reset()
		root.encode(&buf)
		err := json.Unmarshal(buf.Bytes(), reflect.New(typ).Interface(), opts)
		if err == nil {
			break
		}
		var se *json.SemanticError
		if !errors.As(err, &se) {
			problems = append(problems, &FieldError{Err: err})
			break
		}
		_, _, path, _ := root.walk(se.JSONPointer)
		fe := &FieldError{Path: c.pathString(path), Err: valueProblem(se, o.sensitive)}
		if errors.Is(err, json.ErrUnknownName) {
			fe.Err = ErrUnknownField
		}
		problems = append(problems, fe)
		if !root.remove(se.JSONPointer) {
			break
		}
	}
	if len(problems) >= maxStrictProblems {
		problems = append(problems[:maxStrictProblems], &FieldError{Err: errTooManyProblems})
	}
	if len(problems) > 0 {
		sortProblems(problems)
		return &StrictError{Source: o.source, Fields: problems}
	}
	buf.Reset()
	root.encode(&buf)
	if err := json.Unmarshal(buf.Bytes(), v, opts); err != nil {
		return o.wrap(err)
	}
	return nil
}

// sortProblems orders duplicate keys, found while reading the YAML, among
// the problems found while decoding, by path, keeping the order of
// problems with the same path. errTooManyProblems stays last.
func sortProblems(problems []*FieldError) {
	last := len(problems)
	if last > 0 && problems[last-1].Err == errTooManyProblems {
		last--
	}
	sort.SliceStable(problems[:last], func(i, j int) bool { return problems[i].Path < problems[j].Path })
}

// valueProblem says why a value does not fit its field: the error a
// custom unmarshaler (or the number parser) returned, or "expected
// <type>, got <kind>". For a Sensitive value an unmarshaler's message,
// which may quote the value, is replaced by "not a valid <type>".
func valueProblem(se *json.SemanticError, sensitive bool) error {
	want := "a valid value"
	if se.GoType != nil {
		want = se.GoType.String()
	}
	if se.Err != nil {
		if !sensitive {
			return se.Err
		}
		return &redactedError{msg: "not a valid " + want, err: se.Err}
	}
	msg := "expected " + want
	if got := kindName(se.JSONKind); got != "" {
		msg += ", got " + got
	}
	return errors.New(msg)
}

// kindName names a JSON kind for messages, in YAML's terms.
func kindName(k jsontext.Kind) string {
	switch k {
	case 'n':
		return "null"
	case 't', 'f':
		return "a boolean"
	case '"':
		return "a string"
	case '0':
		return "a number"
	case '{':
		return "a mapping"
	case '[':
		return "a list"
	}
	return ""
}

// errDurationText is the problem with a duration that is not Go's
// duration text. It does not quote the value, which may be Sensitive.
var errDurationText = errors.New("expected a duration such as 30s or 1h30m")

var (
	textUnmarshalerType     = reflect.TypeFor[encoding.TextUnmarshaler]()
	jsonUnmarshalerType     = reflect.TypeFor[json.Unmarshaler]()
	jsonUnmarshalerFromType = reflect.TypeFor[json.UnmarshalerFrom]()
)

// strictOptions are the encoding/json/v2 options of strict decoding.
// text holds the YAML text of numbers and booleans by JSON pointer.
func strictOptions(text map[string]string) json.Options {
	return json.JoinOptions(
		json.RejectUnknownMembers(true),
		json.MatchCaseInsensitiveNames(false),
		jsontext.AllowDuplicateNames(false),
		json.WithUnmarshalers(json.JoinUnmarshalers(
			// time.Duration has no JSON form of its own in v2; config
			// files write Go's duration text.
			json.UnmarshalFromFunc(func(dec *jsontext.Decoder, d *time.Duration) error {
				switch dec.PeekKind() {
				case 'n':
					_, err := dec.ReadToken()
					*d = 0
					return err
				case '"':
					tok, err := dec.ReadToken()
					if err != nil {
						return err
					}
					parsed, err := time.ParseDuration(tok.String())
					if err != nil {
						return errDurationText
					}
					*d = parsed
					return nil
				}
				return errDurationText
			}),
			// A number or boolean for a field that takes text is the text
			// written in the document.
			json.UnmarshalFromFunc(func(dec *jsontext.Decoder, v any) error {
				return textUnmarshaler(dec, v, text)
			}),
		)),
	)
}

// textUnmarshaler decodes a JSON number or boolean into v when v is a
// string, or implements encoding.TextUnmarshaler without a JSON method of
// its own, using the YAML text recorded for it in text ("1.10", "0755",
// "True"). Anything else goes to the default decoding.
func textUnmarshaler(dec *jsontext.Decoder, v any, text map[string]string) error {
	switch dec.PeekKind() {
	case '0', 't', 'f':
	default:
		return errors.ErrUnsupported
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return errors.ErrUnsupported
	}
	t := rv.Type()
	if t.Implements(jsonUnmarshalerType) || t.Implements(jsonUnmarshalerFromType) {
		return errors.ErrUnsupported
	}
	isText := t.Implements(textUnmarshalerType)
	if !isText && rv.Elem().Kind() != reflect.String {
		return errors.ErrUnsupported
	}
	tok, err := dec.ReadToken()
	if err != nil {
		return err
	}
	s := tok.String()
	if written, ok := text[string(dec.StackPointer())]; ok {
		s = written
	}
	if isText {
		return v.(encoding.TextUnmarshaler).UnmarshalText([]byte(s))
	}
	rv.Elem().SetString(s)
	return nil
}
