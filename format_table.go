package clix

import (
	"bytes"
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/SCKelemen/clix/v2/internal/table"
	"github.com/SCKelemen/clix/v2/internal/tty"
)

// textTerminal describes where text output goes: a terminal (with its
// width and whether color is on) or anything else.
type textTerminal struct {
	on       bool
	width    int
	color    bool
	ellipsis string
}

// terminalOf inspects w. A terminal whose size cannot be read is taken to
// be 80 columns wide.
func terminalOf(w io.Writer) textTerminal {
	if !tty.IsTerminal(w) {
		return textTerminal{}
	}
	width, _, ok := tty.Size(w)
	if !ok {
		width = 80
	}
	return textTerminal{on: true, width: width, color: tty.ColorEnabled(w), ellipsis: tty.Symbol("…", "...")}
}

// tableRows returns the elements of node when it is a non-empty list of
// objects, which text output prints as a table.
func tableRows(node interface{}) ([]map[string]interface{}, bool) {
	list, ok := node.([]interface{})
	if !ok || len(list) == 0 {
		return nil, false
	}
	rows := make([]map[string]interface{}, len(list))
	for i, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, false
		}
		rows[i] = m
	}
	return rows, true
}

// writeTable prints rows (the elements of data, converted by textNode) as
// a table on a terminal and as tab-separated values otherwise (see
// FormatData and FormatOptions.Columns).
func writeTable(w io.Writer, data interface{}, rows []map[string]interface{}, opts FormatOptions, term textTerminal) error {
	cols := opts.Columns
	if len(cols) == 0 {
		cols = tableColumns(data, rows)
	}
	cells := make([][]string, len(rows))
	for i, row := range rows {
		line := make([]string, len(cols))
		for j, col := range cols {
			if v, ok := lookupTextPath(row, col); ok {
				line[j] = cellText(v)
			}
		}
		cells[i] = line
	}
	var header []string
	if term.on && len(cols) > 0 {
		header = make([]string, len(cols))
		for j, col := range cols {
			header[j] = columnHeader(col)
		}
	}
	topts := table.Options{Terminal: term.on, Width: term.width, Ellipsis: term.ellipsis}
	if term.on && term.color {
		style := opts.HeaderStyle
		topts.Header = func(s string) string {
			if style == nil {
				return "\x1b[1m" + s + "\x1b[0m"
			}
			return style.Render(s)
		}
	}
	return table.Write(w, header, cells, topts)
}

// tableColumns returns the top-level fields of rows in the order the JSON
// output of data has them: struct fields in declaration order and map
// keys sorted. A field missing from the first rows (omitempty) is placed
// after the field it follows where it appears. When data cannot be
// converted to JSON, the fields are sorted.
func tableColumns(data interface{}, rows []map[string]interface{}) []string {
	if cols, ok := jsonFieldOrder(data); ok {
		return cols
	}
	seen := map[string]bool{}
	var cols []string
	for _, row := range rows {
		for k := range row {
			if !seen[k] {
				seen[k] = true
				cols = append(cols, k)
			}
		}
	}
	sort.Strings(cols)
	return cols
}

// jsonFieldOrder reads the JSON form of data, a list, and merges the key
// order of its objects.
func jsonFieldOrder(data interface{}) ([]string, bool) {
	raw, err := marshalForText(data)
	if err != nil {
		return nil, false
	}
	dec := jsontext.NewDecoder(bytes.NewReader(raw), jsontext.AllowDuplicateNames(true), jsontext.AllowInvalidUTF8(true))
	if tok, err := dec.ReadToken(); err != nil || tok.Kind() != '[' {
		return nil, false
	}
	var cols []string
	for dec.PeekKind() != ']' {
		if dec.PeekKind() != '{' {
			if err := dec.SkipValue(); err != nil {
				return nil, false
			}
			continue
		}
		if _, err := dec.ReadToken(); err != nil {
			return nil, false
		}
		var keys []string
		for dec.PeekKind() != '}' {
			tok, err := dec.ReadToken()
			if err != nil {
				return nil, false
			}
			keys = append(keys, tok.String())
			if err := dec.SkipValue(); err != nil {
				return nil, false
			}
		}
		if _, err := dec.ReadToken(); err != nil {
			return nil, false
		}
		cols = mergeFieldOrder(cols, keys)
	}
	return cols, true
}

// mergeFieldOrder adds the keys of one object to cols, keeping the order
// of every object seen so far: a new key goes after the key that precedes
// it in the object and before the known key that follows it. Between
// those, it goes where it sorts, so that keys no object relates (those of
// maps) end up sorted.
func mergeFieldOrder(cols, keys []string) []string {
	prev := -1 // index in cols of the last key of this object placed so far
	for i, k := range keys {
		if at := slices.Index(cols, k); at >= 0 {
			prev = at
			continue
		}
		lo, hi := prev+1, len(cols)
		for _, next := range keys[i+1:] {
			if j := slices.Index(cols, next); j >= 0 {
				hi = max(j, lo)
				break
			}
		}
		at := lo
		for at < hi && cols[at] < k {
			at++
		}
		cols = slices.Insert(cols, at, k)
		prev = at
	}
	return cols
}

// lookupTextPath finds path in node: a key of an object, or a dotted path
// through nested objects, where a list collects the value from each of
// its elements. A key that contains a dot matches as a whole first.
func lookupTextPath(node interface{}, path string) (interface{}, bool) {
	switch n := node.(type) {
	case map[string]interface{}:
		if v, ok := n[path]; ok {
			return v, true
		}
		head, rest, found := strings.Cut(path, ".")
		if !found {
			return nil, false
		}
		v, ok := n[head]
		if !ok {
			return nil, false
		}
		return lookupTextPath(v, rest)
	case []interface{}:
		var out []interface{}
		found := false
		for _, item := range n {
			if v, ok := lookupTextPath(item, path); ok {
				out = append(out, v)
				found = true
			}
		}
		return out, found
	}
	return nil, false
}

// cellText is the text of one table cell (see FormatOptions.Columns).
func cellText(node interface{}) string {
	switch n := node.(type) {
	case nil:
		return ""
	case map[string]interface{}:
		return compactText(n)
	case []interface{}:
		if allScalarNodes(n) {
			values := make([]string, len(n))
			for i, item := range n {
				values[i] = textLine(item)
			}
			return strings.Join(values, ", ")
		}
		return compactText(n)
	}
	return textLine(node)
}

// jsonCellOptions encode nested values in table cells.
var jsonCellOptions = json.JoinOptions(
	jsonV1Semantics,
	json.Deterministic(true),
	jsontext.EscapeForHTML(false),
	jsontext.EscapeForJS(false),
	jsontext.AllowInvalidUTF8(true),
)

// compactText writes a nested object or list from textNode as compact
// JSON; values JSON could not convert are their %v text.
func compactText(node interface{}) string {
	out, err := json.Marshal(jsonReady(node), jsonCellOptions)
	if err != nil {
		return fmt.Sprintf("%v", node)
	}
	return string(out)
}

// jsonReady replaces what textNode could not convert with its %v text.
func jsonReady(node interface{}) interface{} {
	switch n := node.(type) {
	case map[string]interface{}:
		m := make(map[string]interface{}, len(n))
		for k, v := range n {
			m[k] = jsonReady(v)
		}
		return m
	case []interface{}:
		list := make([]interface{}, len(n))
		for i, v := range n {
			list[i] = jsonReady(v)
		}
		return list
	case textFallback:
		return textLine(n)
	case jsonv1.Number:
		return n
	case fmt.Stringer, error:
		return textLine(n) // as text output prints it
	}
	return node
}

// columnHeader turns a column name into a header: words split at
// underscores, hyphens, dots, spaces and case changes, upper-cased
// ("displayName" and "display_name" are "DISPLAY NAME", "userID" is
// "USER ID", "URLPath" is "URL PATH").
func columnHeader(col string) string {
	runes := []rune(col)
	var words []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			words = append(words, string(cur))
			cur = cur[:0]
		}
	}
	for i, r := range runes {
		switch {
		case r == '_' || r == '-' || r == '.' || unicode.IsSpace(r):
			flush()
			continue
		case unicode.IsUpper(r) && len(cur) > 0:
			prev := cur[len(cur)-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower) {
				flush()
			}
		}
		cur = append(cur, r)
	}
	flush()
	return strings.ToUpper(strings.Join(words, " "))
}
