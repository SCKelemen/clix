package clix

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// FormatData formats data to the specified output writer using the given format.
//
// The format is "json", "yaml" or "text" (case-insensitive); anything else
// is treated as "text".
//
// Text output is meant for people:
//
//   - nil prints nothing.
//   - Strings, numbers, booleans, fmt.Stringer and error values print on
//     one line (a Stringer through its String method).
//   - Maps print one "key = value" line per entry, sorted by key. Nested
//     maps are flattened with dotted keys ("go.os = linux"), lists of plain
//     values are joined with ", ", and lists of objects use their index as
//     a key part ("items.0.name = a").
//   - Lists of plain values print one value per line.
//   - Lists of objects (structs or maps) are tables, one row per element
//     and one column per field. When w is a terminal, the table is aligned
//     and has a header, which is bold unless color is off (NO_COLOR,
//     CLICOLOR=0, TERM=dumb), and the widest columns are truncated to fit
//     the terminal. Otherwise (a pipe, a file, a buffer) each row is one
//     line of tab-separated values, with no header, no color and nothing
//     truncated, so scripts can rely on it. The columns are the fields in
//     the order the JSON output has them (struct fields in declaration
//     order, map keys sorted); FormatDataWith can choose them. See
//     FormatOptions.Columns for how cells are written.
//   - Lists that mix objects with other values print each element as a
//     block, separated by a blank line.
//   - Structs and other values are first converted to JSON (as below), so
//     json struct tags, omitempty, omitzero and MarshalJSON methods decide
//     their keys and values; durations inside them print as "1m30s" and
//     times in RFC 3339. When a value cannot be converted as a whole (it
//     holds a channel, a function or a NaN), it is walked field by field
//     with the same json tag rules, so fields tagged json:"-" and
//     unexported fields are never printed; only the parts JSON cannot
//     encode print with fmt's %v.
//
// JSON output is encoded with encoding/json/v2 using the semantics of
// encoding/json (v1), so every value v1 encodes still encodes the same way
// (struct tags, omitempty, ",string", MarshalJSON, embedded fields), except
// for these deliberate differences:
//
//   - time.Duration is an ISO 8601 duration ("PT1M30S") instead of
//     nanoseconds; time.Time is RFC 3339 with fractional seconds only when
//     they are non-zero, for any year.
//   - Nil slices and maps are [] and {}, and a nil []byte is "", instead of
//     null.
//   - <, >, & and U+2028/U+2029 are not escaped (they only need escaping
//     inside HTML or JavaScript).
//
// It is indented by two spaces and followed by a newline, map keys are
// sorted, invalid UTF-8 is replaced with U+FFFD, and nothing is written
// when the value cannot be encoded.
//
// YAML output is encoded with go.yaml.in/yaml/v3, indented by two spaces.
// Struct fields keep YAML's own rules: their keys are the lower-cased field
// names or yaml tags (json tags are ignored), and a time.Duration field is
// written as yaml writes it ("1m30s"). Elsewhere, at the top level and in
// maps, slices, arrays and pointers, a time.Duration is an ISO 8601
// duration ("PT1M30S", as in JSON) and a json.RawMessage is decoded into
// the value it holds. time.Time is RFC 3339 everywhere.
func FormatData(w io.Writer, data interface{}, format string) error {
	return FormatDataWith(w, data, FormatOptions{Format: format})
}

// FormatOptions configure FormatDataWith.
type FormatOptions struct {
	// Format is "json", "yaml" or "text" (case-insensitive); anything
	// else, including "", is "text". See FormatData.
	Format string

	// Columns chooses the columns of a text table (a list of objects; see
	// FormatData), in order. Each is a field's JSON name, or a dotted path
	// such as "owner.login" into nested objects; a path through a list
	// collects the field of every element ("labels.name"). A key that
	// itself contains a dot is matched as a whole first. Empty means every
	// top-level field, in the order of the JSON output. Rows without the
	// field get an empty cell.
	//
	// A cell is the field's text as in "key = value" lines: strings as
	// they are, numbers, booleans, durations as "1m30s", nothing for null,
	// a list of plain values joined with ", ", and nested objects and
	// lists of objects as compact JSON.
	//
	// The header of a column is its name split into words and upper-cased:
	// "displayName", "display_name" and "display.name" become
	// "DISPLAY NAME".
	Columns []string

	// HeaderStyle styles the header of a terminal table, when color is on.
	// nil means bold.
	HeaderStyle TextStyle
}

// FormatDataWith formats data to w as FormatData does, with the options
// in opts.
func FormatDataWith(w io.Writer, data interface{}, opts FormatOptions) error {
	switch strings.ToLower(opts.Format) {
	case FormatJSON:
		return formatJSON(w, data)
	case FormatYAML:
		return formatYAML(w, data)
	default:
		return formatText(w, data, opts)
	}
}

// isoDuration formats d as an ISO 8601 duration of hours, minutes and
// seconds, with fractional seconds when needed: "PT1M30S", "PT1.5S",
// "-PT2H", "PT26H" (hours are not carried into days, whose length ISO 8601
// leaves to the calendar), and "PT0S" for zero.
//
// ISO 8601-1:2019 section 5.5.2.4 (durations) and ISO 8601-2:2019 section
// 4.4.1.9 (negative durations); see also RFC 3339 Appendix A,
// https://datatracker.ietf.org/doc/html/rfc3339#appendix-A
//
// TODO: switch to iso8601.FormatDuration once
// github.com/SCKelemen/clix/v2/iso8601 lands.
func isoDuration(d time.Duration) string {
	if d == 0 {
		return "PT0S"
	}
	var b strings.Builder
	u := uint64(d)
	if d < 0 {
		b.WriteByte('-')
		u = -u // also right for math.MinInt64
	}
	b.WriteString("PT")
	h := u / uint64(time.Hour)
	u -= h * uint64(time.Hour)
	m := u / uint64(time.Minute)
	u -= m * uint64(time.Minute)
	sec, frac := u/uint64(time.Second), u%uint64(time.Second)
	if h > 0 {
		b.WriteString(strconv.FormatUint(h, 10) + "H")
	}
	if m > 0 {
		b.WriteString(strconv.FormatUint(m, 10) + "M")
	}
	if sec > 0 || frac > 0 {
		b.WriteString(strconv.FormatUint(sec, 10))
		if frac > 0 {
			b.WriteString("." + strings.TrimRight(fmt.Sprintf("%09d", frac), "0"))
		}
		b.WriteString("S")
	}
	return b.String()
}

// formatYAML formats data as YAML using go.yaml.in/yaml/v3.
// Close flushes any buffered output, so its error is reported too.
func formatYAML(w io.Writer, data interface{}) error {
	data, err := prepareYAML(data)
	if err != nil {
		return err
	}
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	if err := encoder.Encode(data); err != nil {
		_ = encoder.Close()
		return err
	}
	return encoder.Close()
}

var (
	rawMessageType = reflect.TypeOf(json.RawMessage(nil))
	durationType   = reflect.TypeOf(time.Duration(0))
)

// prepareYAML replaces the values YAML would show badly: a json.RawMessage
// becomes the JSON value it holds (not a list of byte values), and a
// time.Duration becomes an ISO 8601 duration string ("PT1M30S", as in JSON
// output) instead of yaml's "1m30s". It looks through maps, slices, arrays,
// pointers and interfaces, rebuilding only the containers that hold such a
// value (as map[K]interface{} and []interface{}, which YAML encodes the
// same way). Struct fields are left alone, since rebuilding a struct would
// change its YAML keys. time.Time needs nothing: yaml writes it in RFC 3339
// with fractional seconds only when they are non-zero.
func prepareYAML(data interface{}) (interface{}, error) {
	v, changed, err := replaceForYAML(reflect.ValueOf(data))
	if err != nil || !changed {
		return data, err
	}
	return v, nil
}

func replaceForYAML(rv reflect.Value) (interface{}, bool, error) {
	if !rv.IsValid() {
		return nil, false, nil
	}
	switch rv.Type() {
	case rawMessageType:
		return jsonToYAMLValue(rv.Bytes())
	case durationType:
		return isoDuration(time.Duration(rv.Int())), true, nil
	}
	switch rv.Kind() {
	case reflect.Interface, reflect.Pointer:
		if rv.IsNil() {
			return nil, false, nil
		}
		return replaceForYAML(rv.Elem())
	case reflect.Map:
		type entry struct {
			key, value reflect.Value
		}
		var entries []entry
		found := false
		iter := rv.MapRange()
		for iter.Next() {
			v, changed, err := replaceForYAML(iter.Value())
			if err != nil {
				return nil, false, err
			}
			value := iter.Value()
			if changed {
				found = true
				value = reflect.ValueOf(&v).Elem()
			}
			entries = append(entries, entry{iter.Key(), value})
		}
		if !found {
			return nil, false, nil
		}
		anyType := reflect.TypeOf((*interface{})(nil)).Elem()
		out := reflect.MakeMapWithSize(reflect.MapOf(rv.Type().Key(), anyType), len(entries))
		for _, e := range entries {
			out.SetMapIndex(e.key, e.value)
		}
		return out.Interface(), true, nil
	case reflect.Slice, reflect.Array:
		if rv.Kind() == reflect.Slice && rv.IsNil() {
			return nil, false, nil
		}
		items := make([]interface{}, rv.Len())
		found := false
		for i := range items {
			v, changed, err := replaceForYAML(rv.Index(i))
			if err != nil {
				return nil, false, err
			}
			if changed {
				found = true
			} else {
				v = rv.Index(i).Interface()
			}
			items[i] = v
		}
		if !found {
			return nil, false, nil
		}
		return items, true, nil
	}
	return nil, false, nil
}

// jsonToYAMLValue decodes raw JSON for the YAML encoder. Numbers become
// int64, uint64 or float64, so YAML prints them as numbers.
func jsonToYAMLValue(raw []byte) (interface{}, bool, error) {
	if len(raw) == 0 {
		// A nil RawMessage encodes as JSON null.
		return nil, true, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return nil, false, fmt.Errorf("invalid json.RawMessage: %w", err)
	}
	return jsonNumbers(v), true, nil
}

func jsonNumbers(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		for k, e := range x {
			x[k] = jsonNumbers(e)
		}
	case []interface{}:
		for i, e := range x {
			x[i] = jsonNumbers(e)
		}
	case json.Number:
		if i, err := strconv.ParseInt(string(x), 10, 64); err == nil {
			return i
		}
		if u, err := strconv.ParseUint(string(x), 10, 64); err == nil {
			return u
		}
		if f, err := x.Float64(); err == nil {
			return f
		}
		return string(x)
	}
	return v
}

// formatText formats data as plain text (default).
func formatText(w io.Writer, data interface{}, opts FormatOptions) error {
	switch v := data.(type) {
	case map[string]interface{}:
		if scalarMap(v) {
			return formatTextMap(w, v)
		}
	case map[string]string:
		// Convert map[string]string to map[string]interface{}
		m := make(map[string]interface{})
		for k, val := range v {
			m[k] = val
		}
		return formatTextMap(w, m)
	case []interface{}:
		if scalarList(v) {
			return formatTextList(w, v)
		}
	case []string:
		// Convert []string to []interface{}
		list := make([]interface{}, len(v))
		for i, val := range v {
			list[i] = val
		}
		return formatTextList(w, list)
	}
	node := textNode(data)
	if rows, ok := tableRows(node); ok {
		return writeTable(w, data, rows, opts, terminalOf(w))
	}
	return writeTextNode(w, node)
}

func formatTextMap(w io.Writer, m map[string]interface{}) error {
	// Sort keys for consistent output
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if _, err := fmt.Fprintf(w, "%s = %s\n", key, formatValue(m[key])); err != nil {
			return err
		}
	}
	return nil
}

func formatTextList(w io.Writer, list []interface{}) error {
	for _, item := range list {
		if _, err := fmt.Fprintf(w, "%s\n", formatValue(item)); err != nil {
			return err
		}
	}
	return nil
}

// formatValue converts a value to its string representation.
func formatValue(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", val)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val)
	case float32, float64:
		return fmt.Sprintf("%g", val)
	case bool:
		return fmt.Sprintf("%t", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// textFallback holds a value that encoding/json could not convert; text
// output prints it with fmt's %v.
type textFallback struct{ v interface{} }

// isTextScalar reports whether text output prints v on one line as it is:
// nil, a string, number or boolean (including named types of those kinds),
// a fmt.Stringer, or an error.
func isTextScalar(v interface{}) bool {
	switch v.(type) {
	case json.RawMessage: // since Go 1.27 a jsontext.Value, whose String returns raw JSON
		return false
	case nil, fmt.Stringer, error:
		return true
	}
	switch reflect.ValueOf(v).Kind() {
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return true
	}
	return false
}

// scalarMap and scalarList report whether every value is a text scalar,
// so the map or list keeps printing the way it always has.
func scalarMap(m map[string]interface{}) bool {
	for _, v := range m {
		if !isTextScalar(v) {
			return false
		}
	}
	return true
}

func scalarList(list []interface{}) bool {
	for _, v := range list {
		if !isTextScalar(v) {
			return false
		}
	}
	return true
}

// textNode converts v into the tree text output prints: scalars (see
// isTextScalar), map[string]interface{}, []interface{}, or textFallback.
// Maps and slices of the common generic types are walked directly so the
// scalars inside keep printing as before (a Stringer through String);
// anything else goes through encoding/json.
func textNode(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		m := make(map[string]interface{}, len(x))
		for k, e := range x {
			m[k] = textNode(e)
		}
		return m
	case map[string]string:
		m := make(map[string]interface{}, len(x))
		for k, e := range x {
			m[k] = e
		}
		return m
	case []interface{}:
		list := make([]interface{}, len(x))
		for i, e := range x {
			list[i] = textNode(e)
		}
		return list
	case []string:
		list := make([]interface{}, len(x))
		for i, e := range x {
			list[i] = e
		}
		return list
	}
	if isTextScalar(v) {
		return v
	}
	if out, ok := jsonTextNode(v); ok {
		return out
	}
	return fallbackNode(reflect.ValueOf(v), 0)
}

// jsonTextNode converts v through JSON (see jsonTextOptions).
func jsonTextNode(v interface{}) (interface{}, bool) {
	data, err := marshalForText(v)
	if err != nil {
		return nil, false
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var out interface{}
	if err := dec.Decode(&out); err != nil {
		return nil, false
	}
	return out, true
}

// maxFallbackDepth bounds fallbackNode on cyclic values.
const maxFallbackDepth = 32

// fallbackNode converts a value that JSON could not encode as a whole (it
// holds a channel, a function, a NaN, a cycle, ...). It walks the value,
// converting each part through JSON where it can, and follows the json
// struct tag rules for struct fields, so a field tagged json:"-" (or an
// unexported one) is never printed. Only the parts JSON cannot encode at
// all print with %v.
func fallbackNode(rv reflect.Value, depth int) interface{} {
	if !rv.IsValid() {
		return nil
	}
	if depth > maxFallbackDepth {
		return "..."
	}
	if depth > 0 && rv.CanInterface() {
		x := rv.Interface()
		if isTextScalar(x) {
			return x
		}
		if out, ok := jsonTextNode(x); ok {
			return out
		}
	}
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface:
		if rv.IsNil() {
			return nil
		}
		return fallbackNode(rv.Elem(), depth+1)
	case reflect.Map:
		m := make(map[string]interface{}, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			m[fmt.Sprint(iter.Key())] = fallbackNode(iter.Value(), depth+1)
		}
		return m
	case reflect.Slice, reflect.Array:
		list := make([]interface{}, rv.Len())
		for i := range list {
			list[i] = fallbackNode(rv.Index(i), depth+1)
		}
		return list
	case reflect.Struct:
		m := make(map[string]interface{})
		fallbackFields(rv, m, depth)
		return m
	case reflect.Float32, reflect.Float64:
		return rv.Float() // NaN and ±Inf, which JSON cannot encode
	}
	if rv.CanInterface() {
		return textFallback{rv.Interface()}
	}
	// A field promoted from an unexported embedded struct cannot be read
	// through Interface; read plain values by kind.
	switch rv.Kind() {
	case reflect.Bool:
		return rv.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return rv.Uint()
	case reflect.String:
		return rv.String()
	}
	return nil
}

// fallbackFields adds the fields of struct rv that encoding/json would
// encode to m, under their JSON names: exported fields not tagged "-"
// (json:"-," names a field "-"), omitempty fields only when not empty, and
// the fields of untagged embedded structs in place.
func fallbackFields(rv reflect.Value, m map[string]interface{}, depth int) {
	t := rv.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		fv := rv.Field(i)
		if f.Anonymous && name == "" {
			ft := f.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				if fv.Kind() == reflect.Pointer {
					if fv.IsNil() {
						continue
					}
					fv = fv.Elem()
				}
				fallbackFields(fv, m, depth+1)
				continue
			}
		}
		if !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		if _, exists := m[name]; exists {
			continue // a shallower field wins, as in encoding/json
		}
		if hasJSONOption(opts, "omitempty") && isEmptyJSONValue(fv) {
			continue
		}
		if hasJSONOption(opts, "omitzero") && fv.IsZero() {
			continue
		}
		m[name] = fallbackNode(fv, depth+1)
	}
}

func hasJSONOption(opts, option string) bool {
	for _, o := range strings.Split(opts, ",") {
		if o == option {
			return true
		}
	}
	return false
}

// isEmptyJSONValue is encoding/json's omitempty test.
func isEmptyJSONValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Interface, reflect.Pointer:
		return v.IsZero()
	}
	return false
}

// textLine returns the one-line text of a scalar or fallback node.
func textLine(node interface{}) string {
	if f, ok := node.(textFallback); ok {
		return fmt.Sprintf("%v", f.v)
	}
	return formatValue(node)
}

// writeTextNode prints a node from textNode.
func writeTextNode(w io.Writer, node interface{}) error {
	switch n := node.(type) {
	case nil:
		return nil
	case map[string]interface{}:
		var lines []string
		flattenText("", n, &lines)
		return writeLines(w, lines)
	case []interface{}:
		return writeTextList(w, n)
	default:
		_, err := fmt.Fprintf(w, "%s\n", textLine(n))
		return err
	}
}

// writeTextList prints a list: plain values one per line, otherwise each
// item as a block separated by a blank line.
func writeTextList(w io.Writer, items []interface{}) error {
	if allScalarNodes(items) {
		lines := make([]string, len(items))
		for i, item := range items {
			lines[i] = textLine(item)
		}
		return writeLines(w, lines)
	}
	for i, item := range items {
		if i > 0 {
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		if err := writeTextNode(w, item); err != nil {
			return err
		}
	}
	return nil
}

// flattenText appends "key = value" lines for node, with nested keys joined
// by dots.
func flattenText(prefix string, node interface{}, lines *[]string) {
	add := func(value string) { *lines = append(*lines, prefix+" = "+value) }
	switch n := node.(type) {
	case map[string]interface{}:
		if len(n) == 0 && prefix != "" {
			add("")
			return
		}
		keys := make([]string, 0, len(n))
		for k := range n {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			flattenText(joinTextKey(prefix, k), n[k], lines)
		}
	case []interface{}:
		if allScalarNodes(n) {
			values := make([]string, len(n))
			for i, item := range n {
				values[i] = textLine(item)
			}
			add(strings.Join(values, ", "))
			return
		}
		for i, item := range n {
			flattenText(joinTextKey(prefix, strconv.Itoa(i)), item, lines)
		}
	default:
		add(textLine(n))
	}
}

func joinTextKey(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func allScalarNodes(items []interface{}) bool {
	for _, item := range items {
		switch item.(type) {
		case map[string]interface{}, []interface{}:
			return false
		}
	}
	return true
}

func writeLines(w io.Writer, lines []string) error {
	for _, line := range lines {
		if _, err := io.WriteString(w, line+"\n"); err != nil {
			return err
		}
	}
	return nil
}
