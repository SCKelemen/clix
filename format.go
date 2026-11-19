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
//   - Lists of plain values print one value per line; lists of objects
//     print each object as a block of "key = value" lines, separated by a
//     blank line.
//   - Structs and other values are first converted with encoding/json, so
//     json struct tags, omitempty and MarshalJSON methods decide their keys
//     and values. A value encoding/json cannot handle prints with fmt's %v.
func FormatData(w io.Writer, data interface{}, format string) error {
	switch strings.ToLower(format) {
	case FormatJSON:
		return formatJSON(w, data)
	case FormatYAML:
		return formatYAML(w, data)
	default:
		return formatText(w, data)
	}
}

// formatJSON formats data as JSON with indentation.
func formatJSON(w io.Writer, data interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// formatYAML formats data as YAML using go.yaml.in/yaml/v3.
// Close flushes any buffered output, so its error is reported too.
func formatYAML(w io.Writer, data interface{}) error {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	if err := encoder.Encode(data); err != nil {
		_ = encoder.Close()
		return err
	}
	return encoder.Close()
}

// formatText formats data as plain text (default).
func formatText(w io.Writer, data interface{}) error {
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
	return writeTextNode(w, textNode(data))
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
	data, err := json.Marshal(v)
	if err != nil {
		return textFallback{v}
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var out interface{}
	if err := dec.Decode(&out); err != nil {
		return textFallback{v}
	}
	return out
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
