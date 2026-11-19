package format

import (
	"bytes"
	jsonv1 "encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/SCKelemen/clix/v2/internal/table"
	"github.com/SCKelemen/clix/v2/internal/tty"
)

// now is the clock timeago reads; tests replace it.
var now = time.Now

// templateState is what the helpers of one --template execution share.
type templateState struct {
	out      io.Writer
	terminal bool // out is a terminal: tables are truncated to width
	width    int
	color    bool // ANSI color and hyperlinks may be written
	rows     [][]string
}

// parseTemplate parses text with the helper functions bound to st.
func parseTemplate(text string, st *templateState) (*template.Template, error) {
	return template.New("template").Funcs(template.FuncMap{
		"autocolor":   st.colorFunc,
		"color":       st.colorFunc,
		"hyperlink":   st.hyperlink,
		"join":        joinFunc,
		"pluck":       pluckFunc,
		"tablerender": st.tableRender,
		"tablerow":    st.tableRow,
		"timeago":     timeAgoFunc,
		"timefmt":     timeFormatFunc,
		"truncate":    truncateFunc,
	}).Parse(text)
}

// executeTemplate runs a --template over doc, the JSON output, and
// renders any table rows left at the end.
func executeTemplate(out io.Writer, text string, doc []byte) error {
	st := &templateState{out: out}
	if tty.IsTerminal(out) {
		st.terminal = true
		st.width, _, _ = tty.Size(out)
	}
	st.color = tty.ColorEnabled(out)
	tmpl, err := parseTemplate(text, st)
	if err != nil {
		return fmt.Errorf("--template: %w", err)
	}
	data, err := templateData(doc)
	if err != nil {
		return err
	}
	if err := tmpl.Execute(out, data); err != nil {
		return fmt.Errorf("--template: %w", err)
	}
	_, err = st.tableRender()
	return err
}

// templateData decodes JSON for a template: objects as map[string]any,
// lists as []any, whole numbers as int64 (or uint64 when larger), other
// numbers as float64, so {{.count}} prints 3 and {{if gt .count 1}}
// compares numbers.
func templateData(doc []byte) (any, error) {
	dec := jsonv1.NewDecoder(bytes.NewReader(doc))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return templateNumbers(v), nil
}

func templateNumbers(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			x[k] = templateNumbers(e)
		}
	case []any:
		for i, e := range x {
			x[i] = templateNumbers(e)
		}
	case jsonv1.Number:
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

// scalarText is the text of a JSON scalar in a template helper: strings
// as they are, numbers in full, booleans, and "" for null. Objects and
// lists are an error.
func scalarText(v any) (string, error) {
	switch x := v.(type) {
	case nil:
		return "", nil
	case string:
		return x, nil
	case bool:
		return strconv.FormatBool(x), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case uint64:
		return strconv.FormatUint(x, 10), nil
	case int:
		return strconv.Itoa(x), nil
	case float64:
		if x == math.Trunc(x) && math.Abs(x) < 1e21 {
			return strconv.FormatFloat(x, 'f', -1, 64), nil
		}
		return strconv.FormatFloat(x, 'g', -1, 64), nil
	case fmt.Stringer:
		return x.String(), nil
	}
	return "", fmt.Errorf("cannot use %T as text", v)
}

// colorFunc implements color and autocolor: {{color "green+b" .name}}.
// The style is foreground[+attributes][:background[+h]], with the colors
// black, red, green, yellow, blue, magenta, cyan, white, default, or a
// number from 0 to 255, and the attributes b (bold), d (dim), i
// (italic), u (underline), B (blink), r (reverse), s (strikethrough) and
// h (bright). The text is styled only when color is on for the output
// (not with NO_COLOR, CLICOLOR=0, TERM=dumb or a pipe, unless
// CLICOLOR_FORCE is set).
func (st *templateState) colorFunc(style string, input any) (string, error) {
	text, err := scalarText(input)
	if err != nil {
		return "", err
	}
	codes, err := sgrCodes(style)
	if err != nil {
		return "", err
	}
	if !st.color || codes == "" {
		return text, nil
	}
	return "\x1b[" + codes + "m" + text + "\x1b[0m", nil
}

var colorIndex = map[string]int{"black": 0, "red": 1, "green": 2, "yellow": 3, "blue": 4, "magenta": 5, "cyan": 6, "white": 7}

// sgrCodes turns a color style into SGR parameters (ECMA-48 section
// 8.3.117).
func sgrCodes(style string) (string, error) {
	fgSpec, bgSpec, _ := strings.Cut(style, ":")
	var codes []string
	addColor := func(spec string, background bool) error {
		name, attrs, _ := strings.Cut(spec, "+")
		bright := strings.Contains(attrs, "h")
		base := 30
		if background {
			base = 40
		}
		switch {
		case name == "" || name == "default":
			if name == "default" {
				codes = append(codes, strconv.Itoa(base+9))
			}
		case colorIndex[name] != 0 || name == "black":
			c := base + colorIndex[name]
			if bright {
				c += 60
			}
			codes = append(codes, strconv.Itoa(c))
		default:
			n, err := strconv.Atoi(name)
			if err != nil || n < 0 || n > 255 {
				return fmt.Errorf("unknown color %q in style %q", name, style)
			}
			codes = append(codes, strconv.Itoa(base+8), "5", strconv.Itoa(n))
		}
		for _, a := range attrs {
			if background {
				if a != 'h' {
					return fmt.Errorf("unknown background attribute %q in style %q", a, style)
				}
				continue
			}
			switch a {
			case 'b':
				codes = append(codes, "1")
			case 'd':
				codes = append(codes, "2")
			case 'i':
				codes = append(codes, "3")
			case 'u':
				codes = append(codes, "4")
			case 'B':
				codes = append(codes, "5")
			case 'r':
				codes = append(codes, "7")
			case 's':
				codes = append(codes, "9")
			case 'h':
			default:
				return fmt.Errorf("unknown attribute %q in style %q", a, style)
			}
		}
		return nil
	}
	if err := addColor(fgSpec, false); err != nil {
		return "", err
	}
	if bgSpec != "" {
		if err := addColor(bgSpec, true); err != nil {
			return "", err
		}
	}
	return strings.Join(codes, ";"), nil
}

// hyperlink implements {{hyperlink .url .title}}: an OSC 8 hyperlink when
// color is on for the output, the text (or the URL when the text is
// empty) otherwise.
// https://gist.github.com/egmontkob/eb114294efbcd5adb1944c9f3cb5feda
func (st *templateState) hyperlink(link, text string) string {
	if text == "" {
		text = link
	}
	if !st.color || strings.ContainsAny(link, "\x1b\a") {
		return text
	}
	return "\x1b]8;;" + link + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

// joinFunc implements {{join ", " .labels}}.
func joinFunc(sep string, input []any) (string, error) {
	parts := make([]string, len(input))
	for i, v := range input {
		s, err := scalarText(v)
		if err != nil {
			return "", fmt.Errorf("join: %w", err)
		}
		parts[i] = s
	}
	return strings.Join(parts, sep), nil
}

// pluckFunc implements {{pluck "name" .labels}}: the field of every
// object in a list (null when an object lacks it).
func pluckFunc(field string, input []any) ([]any, error) {
	out := make([]any, 0, len(input))
	for _, v := range input {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("pluck: element is not an object")
		}
		out = append(out, m[field])
	}
	return out, nil
}

// tableRow implements {{tablerow .a .b}}: it adds a row to the table
// that tablerender (or the end of the template) prints.
func (st *templateState) tableRow(fields ...any) (string, error) {
	row := make([]string, len(fields))
	for i, f := range fields {
		s, err := scalarText(f)
		if err != nil {
			return "", fmt.Errorf("tablerow: %w", err)
		}
		row[i] = s
	}
	st.rows = append(st.rows, row)
	return "", nil
}

// tableRender implements {{tablerender}}: it prints the rows added so far
// as aligned columns (truncated to the terminal's width on a terminal)
// and starts a new table.
func (st *templateState) tableRender() (string, error) {
	if len(st.rows) == 0 || st.out == nil {
		return "", nil
	}
	width := 0
	if st.terminal {
		width = st.width
	}
	rows := st.rows
	st.rows = nil
	return "", table.Write(st.out, nil, rows, table.Options{Terminal: true, Width: width, Ellipsis: tty.Symbol("…", "...")})
}

// truncateFunc implements {{truncate 20 .title}}: at most that many
// display columns, ending in "..." when cut.
func truncateFunc(length int, input any) (string, error) {
	s, err := scalarText(input)
	if err != nil {
		return "", fmt.Errorf("truncate: %w", err)
	}
	return table.Truncate(s, length, "..."), nil
}

// timeAgoFunc implements {{timeago .createdAt}} for an RFC 3339 time:
// "less than a minute ago", "about 1 hour ago", "about 3 days ago", and
// "in about 2 hours" for a time in the future, as gh words it.
func timeAgoFunc(input any) (string, error) {
	s, err := scalarText(input)
	if err != nil {
		return "", fmt.Errorf("timeago: %w", err)
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return "", fmt.Errorf("timeago: %w", err)
	}
	return relativeTime(now().Sub(t)), nil
}

func relativeTime(ago time.Duration) string {
	future := ago < 0
	if future {
		ago = -ago
	}
	var amount int
	var unit string
	switch {
	case ago < time.Minute:
		if future {
			return "in less than a minute"
		}
		return "less than a minute ago"
	case ago < time.Hour:
		amount, unit = int(ago.Minutes()), "minute"
	case ago < 24*time.Hour:
		amount, unit = int(ago.Hours()), "hour"
	case ago < 30*24*time.Hour:
		amount, unit = int(ago.Hours())/24, "day"
	case ago < 365*24*time.Hour:
		amount, unit = int(ago.Hours())/24/30, "month"
	default:
		amount, unit = int(ago.Hours()/24/365), "year"
	}
	if amount != 1 {
		unit += "s"
	}
	if future {
		return fmt.Sprintf("in about %d %s", amount, unit)
	}
	return fmt.Sprintf("about %d %s ago", amount, unit)
}

// timeFormatFunc implements {{timefmt "2006-01-02" .createdAt}}: an
// RFC 3339 time in a Go time layout.
func timeFormatFunc(layout string, input any) (string, error) {
	s, err := scalarText(input)
	if err != nil {
		return "", fmt.Errorf("timefmt: %w", err)
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return "", fmt.Errorf("timefmt: %w", err)
	}
	return t.Format(layout), nil
}
