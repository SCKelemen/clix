package format

import (
	"bytes"
	"encoding/json/jsontext"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/SCKelemen/clix/v2"
)

// Schema declares the fields of a command's records: the object a
// command prints, or each object in the list it prints. Write applies
// it once, before any output flag runs, to build one canonical
// document: the command's JSON output with every declared field that a
// record lacks filled in (see Field). --format json, yaml and text, the
// gcloud format expressions, --json, --jq, --template, row templates and
// the list flags (--filter, --sort-by, --limit) all read that document,
// so they agree on every field, and Write checks the field names that
// --json and row templates use against the schema.
//
// A Schema describes JSON, not a Go type, so any producer can implement
// it: a struct type for its omitempty fields, a SQL result for its
// columns, or an adapter for another data model, such as a protobuf
// message descriptor, in its own module. A command attaches one to its
// result with Document, or its result implements Schema itself (see
// Write). Without a schema, Write prints what the command gives it.
type Schema interface {
	// Fields returns the fields of a record, in the order the canonical
	// document lists them.
	Fields() []Field
}

// Field is one field of a Schema.
type Field struct {
	// Name is the field's key in the JSON output, such as "displayName"
	// for a struct field tagged json:"displayName".
	Name string

	// Zero is the value a record that lacks the field gets, encoded as
	// clix.FormatData encodes JSON: 0, "", false, []any{} or
	// map[string]any{}, for example. A nil Zero is null, except for a
	// field with nested Fields, which then gets an object of their
	// values. Zero is not used when Nullable is set.
	Zero any

	// Nullable fills a missing field with null, whatever Zero is: a
	// field whose absence means something, such as an optional value or
	// a nested message that was not set.
	Nullable bool

	// Fields, when set, are the fields of the object the field holds, or
	// of each object in the list it holds, filled in the same way. Leave
	// it empty for a map whose keys are data rather than fields.
	Fields []Field
}

// Fields is a Schema given as a list of fields.
//
//	format.Document{Value: repos, Schema: format.Fields{
//		{Name: "name", Zero: ""},
//		{Name: "stars", Zero: 0},
//		{Name: "owner", Nullable: true},
//	}}
type Fields []Field

// Fields implements Schema.
func (f Fields) Fields() []Field { return f }

// Document is a command's result together with the Schema of its
// records, for a result whose type does not implement Schema itself,
// such as json.RawMessage values from another encoder. Write prints the
// canonical document of Value under Schema, and so does clix.FormatData,
// in every format, through Document's MarshalJSON and MarshalYAML
// methods. A nil Schema leaves Value to print as it would on its own.
type Document struct {
	// Value is the command's result: a record, a list of records, or a
	// value that holds them, as Write and clix.FormatData take it.
	Value any

	// Schema declares the fields of the records in Value.
	Schema Schema
}

// MarshalJSON returns the canonical document: the JSON output of Value
// with the fields Schema declares filled in.
func (d Document) MarshalJSON() ([]byte, error) {
	value, s, err := schemaOf(d)
	if err != nil {
		return nil, err
	}
	b, err := canonicalJSON(value, s)
	if err != nil {
		return nil, err
	}
	v := jsontext.Value(b)
	if err := v.Compact(); err != nil {
		return nil, err
	}
	return v, nil
}

// MarshalYAML returns the canonical document as YAML encodes it, the way
// clix.FormatData prints a json.RawMessage: objects as maps, with sorted
// keys, and whole numbers as integers.
func (d Document) MarshalYAML() (any, error) {
	b, err := d.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return templateData(b)
}

// schema is a Schema checked and ready to apply: its fields, with every
// Zero already decoded.
type schema struct {
	fields []schemaField
}

type schemaField struct {
	name     string
	nullable bool
	zero     any // the decoded Zero; nil for a nil Zero with nested fields
	fields   []schemaField
}

// names returns the names of the top-level fields.
func (s *schema) names() []string {
	if s == nil {
		return nil
	}
	names := make([]string, len(s.fields))
	for i, f := range s.fields {
		names[i] = f.name
	}
	return names
}

// schemaOf returns the value data prints and its schema, or a nil schema
// when it has none: a Document's Value and Schema (with a nil Schema, the
// schema of Value), data's own when it implements Schema, or, for a slice
// or array (or a pointer to one), that of its element type when a new
// element implements Schema.
func schemaOf(data any) (any, *schema, error) {
	switch d := data.(type) {
	case Document:
		if d.Schema == nil {
			return schemaOf(d.Value)
		}
		s, err := compileSchema(d.Schema)
		return d.Value, s, err
	case *Document:
		if d != nil {
			return schemaOf(*d)
		}
	case Schema:
		if rv := reflect.ValueOf(d); rv.Kind() == reflect.Pointer && rv.IsNil() {
			break // a nil record: ask a new one, below
		}
		s, err := compileSchema(d)
		return data, s, err
	}
	if d, ok := elemSchema(reflect.TypeOf(data)); ok {
		s, err := compileSchema(d)
		return data, s, err
	}
	return data, nil, nil
}

// elemSchema returns a new value of the type behind t, through pointers,
// or of the element type of the slice or array behind it, when that
// value implements Schema. A pointer is used so that methods with either
// receiver count.
func elemSchema(t reflect.Type) (Schema, bool) {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil {
		return nil, false
	}
	if t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
		for t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
	}
	if t.Kind() == reflect.Interface {
		return nil, false
	}
	s, ok := reflect.New(t).Interface().(Schema)
	return s, ok
}

// compileSchema checks the fields of s and decodes their Zero values.
func compileSchema(s Schema) (*schema, error) {
	fields, err := compileFields(s.Fields(), "")
	if err != nil {
		return nil, err
	}
	return &schema{fields: fields}, nil
}

func compileFields(fields []Field, prefix string) ([]schemaField, error) {
	out := make([]schemaField, 0, len(fields))
	for _, f := range fields {
		path := prefix + f.Name
		if f.Name == "" {
			return nil, fmt.Errorf("format: schema field with no name under %q", strings.TrimSuffix(prefix, "."))
		}
		if slices.ContainsFunc(out, func(g schemaField) bool { return g.name == f.Name }) {
			return nil, fmt.Errorf("format: schema field %q declared twice", path)
		}
		sf := schemaField{name: f.Name, nullable: f.Nullable}
		if !f.Nullable && (f.Zero != nil || len(f.Fields) == 0) {
			var raw bytes.Buffer
			if err := clix.FormatData(&raw, f.Zero, clix.FormatJSON); err != nil {
				return nil, fmt.Errorf("format: zero value of schema field %q: %w", path, err)
			}
			zero, err := decodeTree(raw.Bytes())
			if err != nil {
				return nil, fmt.Errorf("format: zero value of schema field %q: %w", path, err)
			}
			sf.zero = zero
		}
		if len(f.Fields) > 0 {
			nested, err := compileFields(f.Fields, path+".")
			if err != nil {
				return nil, err
			}
			sf.fields = nested
		}
		out = append(out, sf)
	}
	return out, nil
}

// missing returns the value of the field for a record that lacks it.
func (f schemaField) missing() any {
	switch {
	case f.nullable:
		return jsonNull
	case f.zero == nil: // a nil Zero with nested fields
		return &jsonObject{}
	}
	return cloneTree(f.zero)
}

// canonicalJSON returns the canonical document of value under s: its
// JSON output, as clix.FormatData writes it, with the fields s declares
// filled in. A nil s leaves the output as it is.
func canonicalJSON(value any, s *schema) ([]byte, error) {
	var raw bytes.Buffer
	if err := clix.FormatData(&raw, value, clix.FormatJSON); err != nil {
		return nil, err
	}
	if s == nil {
		return raw.Bytes(), nil
	}
	tree, err := decodeTree(raw.Bytes())
	if err != nil {
		return nil, err
	}
	fillRecords(tree, s.fields)
	return encodeTree(tree)
}

// fillRecords fills fields into v when it is an object, or into each
// object in v when it is a list. Null, other values and the elements of
// a list that are not objects are left as they are.
func fillRecords(v any, fields []schemaField) {
	switch x := v.(type) {
	case *jsonObject:
		fillObject(x, fields)
	case []any:
		for _, e := range x {
			if o, ok := e.(*jsonObject); ok {
				fillObject(o, fields)
			}
		}
	}
}

// fillObject adds the fields o lacks, each right after the field
// declared before it (or first), and fills nested fields into the values
// of those that declare them. Members o has keep their order.
func fillObject(o *jsonObject, fields []schemaField) {
	prev := -1
	for _, f := range fields {
		i := o.index(f.name)
		if i < 0 {
			i = prev + 1
			o.insert(i, f.name, f.missing())
		}
		prev = i
		if len(f.fields) > 0 {
			fillRecords(o.members[i].value, f.fields)
		}
	}
}

// recordNodes returns the records of a canonical document: the elements
// of a list, or the document itself.
func recordNodes(tree any) []any {
	if list, ok := tree.([]any); ok {
		return list
	}
	return []any{tree}
}

// recordKeys returns the keys of the objects among nodes, in order.
func recordKeys(nodes []any) []string {
	var keys []string
	for _, n := range nodes {
		if o, ok := n.(*jsonObject); ok {
			for _, m := range o.members {
				if !slices.Contains(keys, m.name) {
					keys = append(keys, m.name)
				}
			}
		}
	}
	return keys
}

// childNodes returns the values of key in the objects among nodes, with
// a list contributing its elements.
func childNodes(nodes []any, key string) []any {
	var out []any
	for _, n := range nodes {
		o, ok := n.(*jsonObject)
		if !ok {
			continue
		}
		if i := o.index(key); i >= 0 {
			if list, ok := o.members[i].value.([]any); ok {
				out = append(out, list...)
			} else {
				out = append(out, o.members[i].value)
			}
		}
	}
	return out
}

func fieldNamed(fields []schemaField, name string) *schemaField {
	for i := range fields {
		if fields[i].name == name {
			return &fields[i]
		}
	}
	return nil
}

// checkPath reports whether a --json path that names a known top-level
// field also names known nested fields: where the schema declares the
// fields of a field's value, the next part of the path must be one of
// them or a key that some object at that place has. A key that contains
// dots is matched whole first, as --json matches it. When the path is
// unknown, checkPath returns the paths it could have named at the place
// it fails.
func (s *schema) checkPath(path string, records []any) (ok bool, available []string) {
	return checkPath(path, "", s.fields, records, true)
}

func checkPath(path, prefix string, fields []schemaField, nodes []any, top bool) (bool, []string) {
	keys := recordKeys(nodes)
	if fieldNamed(fields, path) != nil || slices.Contains(keys, path) {
		return true, nil
	}
	head, rest, found := strings.Cut(path, ".")
	f := fieldNamed(fields, head)
	if f == nil {
		if top || slices.Contains(keys, head) {
			return true, nil // known from the data, which has no schema below it
		}
		var available []string
		for _, g := range fields {
			available = append(available, prefix+g.name)
		}
		for _, k := range keys {
			if !slices.Contains(available, prefix+k) {
				available = append(available, prefix+k)
			}
		}
		slices.Sort(available)
		return false, available
	}
	if !found || len(f.fields) == 0 {
		return true, nil
	}
	return checkPath(rest, prefix+head+".", f.fields, childNodes(nodes, head), false)
}
