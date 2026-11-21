package projection

import (
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"

	"github.com/SCKelemen/clix/v2/internal/resource"
	"github.com/SCKelemen/clix/v2/internal/termesc"
)

// Options configure Print.
type Options struct {
	// UTF8 selects Unicode box-drawing characters for table[box], as
	// gcloud uses when its output encoding is UTF-8; otherwise they are
	// ASCII and text outside ASCII in a table is an error (gcloud would
	// print it as "?").
	UTF8 bool

	// RawControl prints control characters as they are, as gcloud does
	// (though a table counts each of them as a column wide, where gcloud
	// skips ESC [ sequences when TERM names a terminal with ANSI
	// support). By default they are shown as visible escapes (see
	// termesc): in csv, value, flattened and list as termesc.Text writes
	// them, and in a table without the CR of a Windows line ending, since
	// a table splits a cell into lines and pads them.
	RawControl bool
}

// text returns s as the printers write a field: escaped (termesc.Text)
// unless o.RawControl.
func (o Options) text(s string) string {
	if o.RawControl {
		return s
	}
	return termesc.Text(s)
}

// cell returns s as a table writes a cell, a heading or its title: as
// text does, with every CR escaped.
func (o Options) cell(s string) string {
	if o.RawControl {
		return s
	}
	return termesc.Escape(s, termesc.Tab|termesc.Newline)
}

// ErrNeedsProjection is returned (wrapped) for table, csv and value with
// an empty projection, as gcloud's ProjectionRequiredError.
var ErrNeedsProjection = errors.New("requires a non-empty projection")

// Print prints data, the decoded JSON of a command's result, as gcloud's
// resource_printer.Print does with this format: a list prints one record
// per element and anything else as a single record, and empty data
// prints no records. json and yaml are not printed here (see Project).
func (f *Format) Print(w io.Writer, data any, opts Options) error {
	var records []any
	single := false
	if resource.Truthy(data) {
		if list, ok := data.([]any); ok {
			records = list
		} else {
			records, single = []any{data}, true
		}
	}
	switch f.Name {
	case "table":
		return f.printTable(w, records, opts)
	case "csv", "value":
		return f.printCSV(w, records, opts)
	case "flattened":
		return f.printFlattened(w, records, single, opts)
	case "list":
		return f.printList(w, records, opts)
	case "none":
		return nil
	}
	return fmt.Errorf("projection: %s is printed with Project", f.Name)
}

// Project returns what the json and yaml formats print: the projected
// records, as a list (a single record by itself), and false when gcloud
// prints no records (empty data), which JSON prints as [] and YAML as
// nothing.
func (f *Format) Project(data any) (any, bool) {
	if !resource.Truthy(data) {
		return []any{}, false
	}
	list, ok := data.([]any)
	if !ok {
		return f.evaluate(data, true), true
	}
	out := make([]any, len(list))
	for i, r := range list {
		out[i] = f.evaluate(r, true)
	}
	return out, true
}

func (f *Format) attr(name string) (attrValue, bool) {
	v, ok := f.attrs[name]
	return v, ok
}

func (f *Format) str(name, def string) string {
	if v, ok := f.attrs[name]; ok {
		return v.str
	}
	return def
}

// labels is ProjectionSpec.Labels: the column labels, or nil when all
// are empty.
func (f *Format) labels() []string {
	var labels []string
	any := false
	for _, c := range f.Columns {
		labels = append(labels, c.Label)
		any = any || c.Label != ""
	}
	if !any {
		return nil
	}
	return labels
}

func (f *Format) needColumns() error {
	if len(f.Columns) == 0 {
		return fmt.Errorf("format [%s] %w; give one, such as %s(name, zone)", f.Name, ErrNeedsProjection, f.Name)
	}
	return nil
}

type boxChars struct{ dl, dr, h, hd, hu, ul, ur, v, vh, vl, vr string }

var (
	unicodeBox = boxChars{"┐", "┌", "─", "┬", "┴", "┘", "└", "│", "┼", "┤", "├"}
	asciiBox   = boxChars{"+", "+", "-", "+", "+", "+", "+", "|", "+", "+", "+"}
)

// printTable is table_printer.TablePrinter (AddRecord and Finish),
// without wrapping, optional columns or subformats.
func (f *Format) printTable(w io.Writer, records []any, opts Options) error {
	if err := f.needColumns(); err != nil {
		return err
	}
	var rows [][]any
	for _, r := range records {
		row, err := f.row(r)
		if err != nil {
			return err
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil
	}
	rows = f.sortRows(rows)

	_, allBox := f.attr("all-box")
	_, isBox := f.attr("box")
	var box *boxChars
	pad := 2
	if allBox || isBox {
		box = &asciiBox
		if opts.UTF8 {
			box = &unicodeBox
		}
		pad = 1
	} else if v, ok := f.attr("pad"); ok {
		pad = v.num
	}

	cells := make([][]string, len(rows))
	for i, row := range rows {
		cells[i] = make([]string, len(row))
		for j, v := range row {
			cells[i][j] = opts.cell(stringify(v))
		}
	}
	var heading []string
	if _, ok := f.attr("no-heading"); !ok {
		for _, l := range f.labels() {
			heading = append(heading, opts.cell(l))
		}
	}
	title, hasTitle := f.attrs["title"]
	title.str = opts.cell(title.str)
	texts := slices.Concat(heading, []string{title.str})
	for _, row := range cells {
		texts = append(texts, row...)
	}
	for _, s := range texts {
		if !opts.UTF8 && strings.IndexFunc(s, func(r rune) bool { return r > 0x7f }) >= 0 {
			return resource.Unsupportedf("text outside ASCII in a table when the output is not UTF-8")
		}
	}

	widths := make([]int, len(f.Columns))
	for _, row := range cells {
		for i, c := range row {
			widths[i] = max(widths[i], displayWidth(c))
		}
	}
	for i, c := range heading {
		widths[i] = max(widths[i], displayWidth(c))
	}
	justify := make([]func(string, int) string, len(f.Columns))
	for i, c := range f.Columns {
		switch c.Align {
		case "center":
			justify[i] = center
		case "right":
			justify[i] = rjust
		default:
			justify[i] = ljust
		}
	}

	var b strings.Builder
	if hasTitle {
		var line string
		if box != nil {
			line = box.dr
		}
		width, sep := 0, 2
		for _, cw := range widths {
			width += cw
			if box != nil {
				line += strings.Repeat(box.h, cw+sep)
			}
			sep = 3
		}
		if tw := displayWidth(title.str); width < tw {
			n := len(widths)
			extra := (tw + n - 1) / n
			width += n * extra
			if box != nil {
				line += strings.Repeat(box.h, n*extra)
			}
			for i := range widths {
				widths[i] += extra
			}
		}
		if box != nil {
			width += 3*len(widths) - 1
			line += box.dl
			b.WriteString(line + "\n")
			line = box.v + center(title.str, width) + box.v
		} else {
			width += pad * (len(widths) - 1)
			line = rstripSpace(center(title.str, width))
		}
		b.WriteString(line + "\n")
	}
	var tRule, mRule, bRule string
	all := cells
	if box != nil {
		tSep, mSep, bSep := box.dr, box.vr, box.ur
		if hasTitle {
			tSep = box.vr
		}
		for _, cw := range widths {
			cell := strings.Repeat(box.h, cw+2)
			tRule += tSep + cell
			tSep = box.hd
			mRule += mSep + cell
			mSep = box.vh
			bRule += bSep + cell
			bSep = box.hu
		}
		if hasTitle {
			tRule += box.vl
		} else {
			tRule += box.dl
		}
		mRule += box.vl
		bRule += box.ul
		b.WriteString(tRule + "\n")
		if heading != nil {
			parts := []string{}
			for i, h := range heading {
				parts = append(parts, box.v, pyCenter(h, widths[i]))
			}
			parts = append(parts, box.v)
			b.WriteString(strings.Join(parts, " ") + "\n" + mRule + "\n")
		}
	} else if heading != nil {
		all = append([][]string{slices.Clone(heading)}, cells...)
	}
	for r, row := range all {
		if r > 0 && box != nil && allBox {
			b.WriteString(mRule + "\n")
		}
		row = slices.Clone(row)
		for finished := false; !finished; {
			finished = true
			spaces := 0
			for i := range row {
				cw := widths[i]
				if box != nil {
					b.WriteString(box.v + " ")
				}
				s := row[i]
				var cell string
				if displayWidth(s) > cw || strings.Contains(s, "\n") {
					var rest string
					cell, rest = nextLine(s, cw)
					row[i] = rest
					if rest != "" {
						finished = false
					}
				} else {
					cell = s
					row[i] = " "
				}
				value := justify[i](cell, cw)
				switch {
				case box != nil:
					b.WriteString(value + strings.Repeat(" ", pad))
				case stripSpace(value) != "":
					if spaces > 0 {
						b.WriteString(strings.Repeat(" ", spaces))
					}
					stripped := rstripSpace(value)
					b.WriteString(stripped)
					spaces = pad + displayWidth(value) - displayWidth(stripped)
				default:
					spaces += pad + displayWidth(value)
				}
			}
			if box != nil {
				b.WriteString(box.v)
			}
			b.WriteString("\n")
		}
	}
	if box != nil {
		b.WriteString(bRule + "\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// sortRows sorts rows by the columns with :sort= or :reverse
// (ProjectionSpec.Order and TablePrinter.Finish): columns with the same
// direction form groups, applied from the last to the first; within a
// group each key, from the last, sorts by number (non-numbers after all
// numbers), then by text, then by position.
func (f *Format) sortRows(rows [][]any) [][]any {
	type ordered struct {
		order   int
		index   int
		reverse bool
	}
	var ordering []ordered
	for i, c := range f.Columns {
		if c.Sort > 0 || c.Reverse {
			o := c.Sort
			if o == 0 {
				o = 1 << 62
			}
			ordering = append(ordering, ordered{o, i, c.Reverse})
		}
	}
	if len(ordering) == 0 {
		return rows
	}
	slices.SortStableFunc(ordering, func(a, b ordered) int {
		if a.order != b.order {
			return a.order - b.order
		}
		return a.index - b.index
	})
	type group struct {
		keys    []int
		reverse bool
	}
	var groups []group
	var keys []int
	reverse := false
	for _, o := range ordering {
		if len(keys) == 0 {
			reverse = o.reverse
		}
		if reverse != o.reverse {
			groups = append([]group{{keys, reverse}}, groups...)
			keys = nil
			reverse = o.reverse
		}
		keys = append(keys, o.index)
	}
	if len(keys) > 0 {
		groups = append([]group{{keys, reverse}}, groups...)
	}
	rows = slices.Clone(rows)
	for _, g := range groups {
		if g.reverse {
			slices.Reverse(rows)
		}
		for k := len(g.keys) - 1; k >= 0; k-- {
			key := g.keys[k]
			idx := make([]int, len(rows))
			for i := range idx {
				idx[i] = i
			}
			slices.SortStableFunc(idx, func(a, b int) int {
				if c := compareNumify(rows[a][key], rows[b][key]); c != 0 {
					return c
				}
				return strings.Compare(stringify(rows[a][key]), stringify(rows[b][key]))
			})
			sorted := make([][]any, len(rows))
			for i, j := range idx {
				sorted[i] = rows[j]
			}
			rows = sorted
		}
		if g.reverse {
			slices.Reverse(rows)
		}
	}
	return rows
}

// compareNumify compares table_printer._Numify of two values: numbers by
// value, anything else as infinity.
func compareNumify(a, b any) int {
	c, ok := numify(a).Compare(numify(b))
	if !ok {
		return 0 // NaN, which JSON cannot hold
	}
	return c
}

var infinity = resource.Number{Float: math.Inf(1)}

func numify(v any) resource.Number {
	if n, ok := resource.NumberOf(v); ok {
		return n
	}
	return infinity
}

// printCSV is csv_printer.CsvPrinter and ValuePrinter.
func (f *Format) printCSV(w io.Writer, records []any, opts Options) error {
	if err := f.needColumns(); err != nil {
		return err
	}
	isValue := f.Name == "value"
	delimiter := f.str("delimiter", ";")
	separator := f.str("separator", ",")
	terminator := f.str("terminator", "\n")
	quote := `"`
	if isValue {
		separator = f.str("separator", "\t")
		quote = ""
		if _, ok := f.attr("quote"); ok {
			quote = `"`
		}
	} else if _, ok := f.attr("no-quote"); ok {
		quote = ""
	}
	quoteField := func(field string) string {
		field = opts.text(field)
		if field == "" || quote == "" {
			return field
		}
		r := []rune(field)
		special := strings.Contains(field, delimiter) || strings.Contains(field, quote) || strings.Contains(field, separator) ||
			strings.Contains(field, terminator) || isSpace(r[0]) || isSpace(r[len(r)-1])
		if !special {
			return field
		}
		return quote + strings.ReplaceAll(field, quote, quote+quote) + quote
	}
	var b strings.Builder
	headingDone := isValue
	for _, r := range records {
		row, err := f.row(r)
		if err != nil {
			return err
		}
		if !headingDone {
			headingDone = true
			if _, ok := f.attr("no-heading"); !ok {
				if labels := f.labels(); labels != nil {
					fields := make([]string, len(labels))
					for i, l := range labels {
						fields[i] = quoteField(resource.Lower(l))
					}
					b.WriteString(strings.Join(fields, separator) + terminator)
				}
			}
		}
		line := make([]string, len(row))
		for i, col := range row {
			switch col := col.(type) {
			case nil:
			case *resource.Object:
				keys := slices.Clone(col.Keys())
				slices.Sort(keys)
				parts := make([]string, len(keys))
				for j, k := range keys {
					v, _ := col.Get(k)
					parts[j] = quoteField(k + "=" + resource.Str(v))
				}
				line[i] = strings.Join(parts, delimiter)
			case []any:
				parts := make([]string, len(col))
				for j, e := range col {
					if resource.Truthy(e) {
						parts[j] = quoteField(resource.Str(e))
					}
				}
				line[i] = strings.Join(parts, delimiter)
			case float64:
				line[i] = quoteField(floatText(col))
			default:
				line[i] = quoteField(resource.Str(col))
			}
		}
		b.WriteString(strings.Join(line, separator) + terminator)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// printFlattened is flattened_printer.FlattenedPrinter.
func (f *Format) printFlattened(w io.Writer, records []any, single bool, opts Options) error {
	var labels map[string]string
	if len(f.Columns) > 0 {
		labels = map[string]string{}
		for _, c := range f.Columns {
			labels[keyName(c.Key)] = c.Label
		}
	}
	separator := f.str("separator", ": ")
	_, noPad := f.attr("no-pad")
	var b strings.Builder
	for _, r := range records {
		record := f.evaluate(r, false)
		if !single {
			b.WriteString("---\n")
		}
		type pair struct {
			key   string
			value any
		}
		var pairs []pair
		add := func(name string, v any) {
			if l, ok := labels[name]; ok {
				name = resource.Lower(l)
			}
			pairs = append(pairs, pair{name, v})
		}
		var flatten func(v any, name string)
		flatten = func(v any, name string) {
			switch v := v.(type) {
			case []any:
				if len(v) == 0 {
					add(name, v)
				}
				for i, e := range v {
					flatten(e, fmt.Sprintf("%s[%d]", name, i))
				}
			case *resource.Object:
				if v.Len() == 0 {
					add(name, v)
				}
				keys := slices.Clone(v.Keys())
				slices.Sort(keys)
				for _, k := range keys {
					e, _ := v.Get(k)
					dot := ""
					if name != "" {
						dot = "."
					}
					flatten(e, name+dot+k)
				}
			case float64:
				add(name, floatText(v))
			default:
				add(name, v)
			}
		}
		flatten(record, "")
		maxKey := 0
		for i, p := range pairs {
			pairs[i].key = opts.text(p.key)
			maxKey = max(maxKey, runeLen(pairs[i].key))
		}
		for _, p := range pairs {
			b.WriteString(p.key + separator)
			if !noPad {
				b.WriteString(strings.Repeat(" ", maxKey-runeLen(p.key)))
			}
			val := resource.Str(p.value)
			r := []rune(val)
			if strings.Contains(val, "\n") || (len(r) > 0 && (isSpace(r[0]) || isSpace(r[len(r)-1]))) {
				val = stringQuote(val)
			}
			b.WriteString(opts.text(val) + "\n")
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// stringQuote is flattened_printer._StringQuote: s in double quotes with
// backslash, the quote, \f, \n, \r and \t escaped.
func stringQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\', '"':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// printList is list_printer.ListPrinter.
func (f *Format) printList(w io.Writer, records []any, opts Options) error {
	separator := "\n   "
	if _, ok := f.attr("compact"); ok {
		separator = " "
	}
	var b strings.Builder
	title, hasTitle := f.attrs["title"]
	if hasTitle && title.str != "" {
		if _, ok := f.attr("always-display-title"); ok {
			b.WriteString(opts.text(title.str) + "\n")
			hasTitle = false
		}
	}
	for _, r := range records {
		var items []string
		switch r.(type) {
		case *resource.Object, []any:
			var record any
			if len(f.Columns) > 0 {
				row, err := f.row(r)
				if err != nil {
					return err
				}
				record = toList(row)
			} else {
				record = f.evaluate(r, false)
			}
			switch rec := record.(type) {
			case *resource.Object:
				keys := slices.Clone(rec.Keys())
				slices.Sort(keys)
				for _, k := range keys {
					v, _ := rec.Get(k)
					if v != nil {
						items = append(items, k+": "+resource.Str(v))
					}
				}
			case []any:
				for _, v := range rec {
					if v != nil {
						items = append(items, resource.Str(v))
					}
				}
			default:
				items = []string{listScalar(rec)}
			}
		default:
			items = []string{listScalar(r)}
		}
		if hasTitle && title.str != "" {
			b.WriteString(opts.text(title.str) + "\n")
			hasTitle = false
		}
		for i, item := range items {
			items[i] = opts.text(item)
		}
		b.WriteString(" - " + strings.Join(items, separator) + "\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func toList(row []any) []any { return row }

// listScalar is encoding.Decode(record or ”): nothing for a false value,
// Python's str() otherwise.
func listScalar(v any) string {
	if !resource.Truthy(v) {
		return ""
	}
	return resource.Str(v)
}
