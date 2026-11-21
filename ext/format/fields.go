package format

import (
	"bytes"
	"encoding"
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"reflect"
	"slices"
	"strings"
	"unicode"
)

// decodeOptions read the JSON that clix.FormatData writes, which may
// repeat a name (a MarshalJSON method can) and replaces invalid UTF-8.
var decodeOptions = jsonv2.JoinOptions(
	jsontext.AllowDuplicateNames(true),
	jsontext.AllowInvalidUTF8(true),
)

// encodeOptions write JSON exactly as clix.FormatData does: indented by
// two spaces, with &, <, > and U+2028/U+2029 as they are.
var encodeOptions = jsonv2.JoinOptions(
	decodeOptions,
	jsontext.WithIndent("  "),
	jsontext.EscapeForHTML(false),
	jsontext.EscapeForJS(false),
)

// availableFields returns, sorted, the fields --json can select from doc,
// the JSON output of data: the keys of the object doc holds, or of the
// objects in the list it holds, and the JSON names of the fields of the
// struct type behind data (see structFields).
func availableFields(doc []byte, data any) ([]string, error) {
	seen := map[string]bool{}
	var fields []string
	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			fields = append(fields, name)
		}
	}
	dec := jsontext.NewDecoder(bytes.NewReader(doc), decodeOptions)
	switch dec.PeekKind() {
	case '{':
		keys, err := objectKeys(dec)
		if err != nil {
			return nil, err
		}
		for _, k := range keys {
			add(k)
		}
	case '[':
		if _, err := dec.ReadToken(); err != nil {
			return nil, err
		}
		for dec.PeekKind() != ']' {
			if dec.PeekKind() != '{' {
				if err := dec.SkipValue(); err != nil {
					return nil, err
				}
				continue
			}
			keys, err := objectKeys(dec)
			if err != nil {
				return nil, err
			}
			for _, k := range keys {
				add(k)
			}
		}
	}
	for _, name := range structFields(reflect.TypeOf(data)) {
		add(name)
	}
	slices.Sort(fields)
	return fields, nil
}

// objectKeys reads one object from dec and returns its keys in order.
func objectKeys(dec *jsontext.Decoder) ([]string, error) {
	if _, err := dec.ReadToken(); err != nil {
		return nil, err
	}
	var keys []string
	for dec.PeekKind() != '}' {
		tok, err := dec.ReadToken()
		if err != nil {
			return nil, err
		}
		keys = append(keys, tok.String())
		if err := dec.SkipValue(); err != nil {
			return nil, err
		}
	}
	_, err := dec.ReadToken()
	return keys, err
}

// fieldKnown reports whether --json field f selects one of the available
// fields: f itself (a key may contain dots), or the part of a dotted path
// before its first dot. The parts after it select nested fields where
// they exist.
func fieldKnown(f string, available []string) bool {
	if slices.Contains(available, f) {
		return true
	}
	head, _, _ := strings.Cut(f, ".")
	return slices.Contains(available, head)
}

// fieldTree is the set of --json fields below one level of the output.
type fieldTree struct {
	whole    map[string]bool       // keys kept with all they hold
	children map[string]*fieldTree // keys of which only some fields are kept
	heads    []string              // the first part of each path, in the order given
	paths    map[string][]string   // the dotted paths given for each head
}

func newFieldTree(fields []string) *fieldTree {
	t := &fieldTree{whole: map[string]bool{}, children: map[string]*fieldTree{}, paths: map[string][]string{}}
	for _, f := range fields {
		t.add(f)
	}
	return t
}

// add records path. A key that is kept whole is also matched by the full
// remaining path, so a key that itself contains dots can be selected.
func (t *fieldTree) add(path string) {
	t.whole[path] = true
	head, rest, found := strings.Cut(path, ".")
	if !slices.Contains(t.heads, head) {
		t.heads = append(t.heads, head)
	}
	if !found {
		return
	}
	t.paths[head] = append(t.paths[head], path)
	child := t.children[head]
	if child == nil {
		child = &fieldTree{whole: map[string]bool{}, children: map[string]*fieldTree{}, paths: map[string][]string{}}
		t.children[head] = child
	}
	child.add(rest)
}

// headOf returns the head that the key name of an object selects, or ""
// when the key is not selected.
func (t *fieldTree) headOf(name string) string {
	if t.children[name] != nil {
		return name
	}
	if t.whole[name] {
		head, _, _ := strings.Cut(name, ".")
		return head
	}
	return ""
}

// selectFields keeps only fields in doc (see Write). An object that
// lacks a selected key gets it with the value null; for a dotted path,
// the first part the object lacks is null. The keys keep the order of
// the full output, and a key added as null goes after the selected key
// before it in the order of the objects that have them.
func selectFields(doc []byte, fields []string) ([]byte, error) {
	tree, err := decodeTree(doc)
	if err != nil {
		return nil, err
	}
	t := newFieldTree(fields)
	order := map[*fieldTree][]string{}
	t.collectOrder(tree, order)
	return encodeTree(t.apply(tree, order))
}

// collectOrder records, for t and the trees below it, the order in which
// the objects of v have the selected keys, merged over every object.
func (t *fieldTree) collectOrder(v any, order map[*fieldTree][]string) {
	switch x := v.(type) {
	case *jsonObject:
		var keys []string
		for _, m := range x.members {
			head := t.headOf(m.name)
			if head == "" {
				continue
			}
			if !slices.Contains(keys, head) {
				keys = append(keys, head)
			}
			if child := t.children[m.name]; child != nil && !t.whole[m.name] {
				child.collectOrder(m.value, order)
			}
		}
		order[t] = mergeKeyOrder(order[t], keys)
	case []any:
		for _, e := range x {
			t.collectOrder(e, order)
		}
	}
}

// mergeKeyOrder adds the keys of one object to order, keeping the order
// of every object seen so far: a new key goes after the key that precedes
// it in the object and before the known key that follows it, as
// clix.FormatData orders the columns of a table.
func mergeKeyOrder(order, keys []string) []string {
	prev := -1 // index in order of the last key of this object placed so far
	for i, k := range keys {
		if at := slices.Index(order, k); at >= 0 {
			prev = at
			continue
		}
		lo, hi := prev+1, len(order)
		for _, next := range keys[i+1:] {
			if j := slices.Index(order, next); j >= 0 {
				hi = max(j, lo)
				break
			}
		}
		at := lo
		for at < hi && order[at] < k {
			at++
		}
		order = slices.Insert(order, at, k)
		prev = at
	}
	return order
}

// apply returns v with only the keys t selects in its objects, applying
// t to every element of a list, and with null for the selected keys an
// object lacks.
func (t *fieldTree) apply(v any, order map[*fieldTree][]string) any {
	switch x := v.(type) {
	case *jsonObject:
		out := &jsonObject{}
		for _, m := range x.members {
			switch {
			case t.whole[m.name]:
				out.members = append(out.members, m)
			case t.children[m.name] != nil:
				out.members = append(out.members, jsonMember{m.name, t.children[m.name].apply(m.value, order)})
			}
		}
		t.fillNull(out, order[t])
		return out
	case []any:
		list := make([]any, len(x))
		for i, e := range x {
			list[i] = t.apply(e, order)
		}
		return list
	}
	return v
}

// fillNull adds null for each head o lacks (by its own name or by a
// dotted path that is a key of o), after the head before it in seen, the
// order of the objects that have them, or else in the order given.
func (t *fieldTree) fillNull(o *jsonObject, seen []string) {
	order := seen
	for _, h := range t.heads {
		if !slices.Contains(order, h) {
			order = append(slices.Clip(order), h)
		}
	}
	prev := -1
	for _, h := range order {
		i := o.index(h)
		for _, p := range t.paths[h] {
			if i >= 0 {
				break
			}
			i = o.index(p)
		}
		if i < 0 {
			i = prev + 1
			o.insert(i, h, jsonNull)
		}
		prev = i
	}
}

var (
	marshalerType     = reflect.TypeFor[jsonv1.Marshaler]()
	marshalerToType   = reflect.TypeFor[jsonv2.MarshalerTo]()
	textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()
)

// structFields returns the JSON names of the fields of the struct type
// behind t (through pointers, and the elements of slices and arrays), as
// encoding/json names them: the json tag's name or the field name,
// exported fields only, "-" skipped, the fields of untagged embedded
// structs promoted, and of several fields with one name the shallowest
// (or the only tagged one among the shallowest) kept. Types that marshal
// themselves have no fields here.
func structFields(t reflect.Type) []string {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t != nil && (t.Kind() == reflect.Slice || t.Kind() == reflect.Array) {
		t = t.Elem()
		for t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
	}
	if t == nil || t.Kind() != reflect.Struct || marshalsItself(t) {
		return nil
	}

	type field struct {
		name   string
		tagged bool
		depth  int
	}
	var fields []field
	visited := map[reflect.Type]bool{}
	next := []reflect.Type{t}
	for depth := 0; len(next) > 0; depth++ {
		current := next
		next = nil
		for _, typ := range current {
			if visited[typ] {
				continue
			}
			visited[typ] = true
			for i := 0; i < typ.NumField(); i++ {
				sf := typ.Field(i)
				ft := sf.Type
				if ft.Name() == "" && ft.Kind() == reflect.Pointer {
					ft = ft.Elem()
				}
				if sf.Anonymous {
					if !sf.IsExported() && ft.Kind() != reflect.Struct {
						continue
					}
				} else if !sf.IsExported() {
					continue
				}
				tag := sf.Tag.Get("json")
				if tag == "-" {
					continue
				}
				name, _, _ := strings.Cut(tag, ",")
				if !validTagName(name) {
					name = ""
				}
				if name == "" && sf.Anonymous && ft.Kind() == reflect.Struct && !marshalsItself(ft) {
					next = append(next, ft)
					continue
				}
				if !sf.IsExported() {
					continue
				}
				tagged := name != ""
				if name == "" {
					name = sf.Name
				}
				fields = append(fields, field{name, tagged, depth})
			}
		}
	}

	var names []string
	for i, f := range fields {
		if slices.ContainsFunc(fields[:i], func(g field) bool { return g.name == f.name }) {
			continue // decided at its first occurrence
		}
		var best []field
		for _, g := range fields {
			if g.name != f.name {
				continue
			}
			if len(best) == 0 || g.depth < best[0].depth {
				best = []field{g}
			} else if g.depth == best[0].depth {
				best = append(best, g)
			}
		}
		if len(best) > 1 {
			var tagged []field
			for _, g := range best {
				if g.tagged {
					tagged = append(tagged, g)
				}
			}
			best = tagged
		}
		if len(best) == 1 {
			names = append(names, f.name)
		}
	}
	return names
}

// marshalsItself reports whether values of t (or *t) encode through a
// method rather than field by field.
func marshalsItself(t reflect.Type) bool {
	for _, m := range []reflect.Type{marshalerType, marshalerToType, textMarshalerType} {
		if t.Implements(m) || reflect.PointerTo(t).Implements(m) {
			return true
		}
	}
	return false
}

// validTagName is encoding/json's rule for the name in a json tag.
func validTagName(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		switch {
		case strings.ContainsRune("!#$%&()*+-./:;<=>?@[]^_{|}~ ", c):
		case !unicode.IsLetter(c) && !unicode.IsDigit(c):
			return false
		}
	}
	return true
}
