package format

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/internal/jq"
	"github.com/SCKelemen/clix/v2/internal/projection"
	"github.com/SCKelemen/clix/v2/internal/tty"
)

// Write prints data, a command's result, to ctx.App.Out as the output
// flags ask. When the running command has list flags (see ListFlags),
// Write first keeps the elements --filter selects, sorts them by
// --sort-by and keeps the first --limit of them; the output flags then
// print what is left:
//
//   - With none of --json, --jq and --template, it is
//     clix.FormatData(out, data, OutputFormat(app)): text by default
//     (a list of objects is a table on a terminal and tab-separated values
//     otherwise), or --format json / yaml.
//   - --json prints the JSON output of clix.FormatData. With fields
//     ("--json name,owner.login"), each object, or each object in a list,
//     keeps only those fields, in the order the full output has them; a
//     dotted path keeps a nested field and works through lists. With no
//     fields ("--json" alone or "--json="), Write returns a usage error
//     that lists the available fields, as does a field the data does not
//     have. The available fields are the keys of the object, or of the
//     objects in the list, plus the JSON names of the fields of the struct
//     type behind them (so an empty list and omitempty fields have them
//     too).
//   - --jq runs a jq expression over that JSON (all of it, or the --json
//     fields) and prints each result on its own line: a string as its
//     raw text, as gh and "jq -r" print it, and any other value as jq
//     prints it, indented by two spaces. In a pipe or a file the output
//     is byte for byte what "jq -r EXPR" prints for the --json output; on
//     a terminal, control characters in a string result print as visible
//     escapes, as in text output (see clix.EscapeControlChars). Only a
//     strict subset of jq is implemented in clix (no dependencies); see
//     the package documentation.
//   - --template executes a Go text/template with the decoded JSON as its
//     data (objects are map[string]any, lists []any, whole numbers int64
//     or uint64, other numbers float64), with the helpers listed in the
//     package documentation. On a terminal, control characters in the
//     data's strings and keys are escaped before the template runs (tab
//     and newline excepted), so the data cannot write escape sequences;
//     the template's own text and the color and hyperlink helpers can.
//
// Text output, gcloud format expressions, --jq on a terminal and
// --template on a terminal escape control characters unless
// opts.RawControlChars is set (see clix.FormatOptions).
//
// With Extension.Projections, a gcloud format expression in --format
// (anything but json, yaml or text alone) prints data as gcloud's printer
// of that name would (see the package documentation); json and yaml with
// a projection print the projected data with clix's encoders. opts.Columns
// are the columns of table, csv and value without a projection.
//
// --jq and --template cannot be combined, and --json, --jq or --template
// with --format yaml or text, or any gcloud format expression, on the
// command line is a usage error (--format json is fine). A --format from
// the environment or the config file gives way to them.
//
// Usage errors are returned before anything is written. A jq or template
// error while running is returned after the output produced before it.
func Write(ctx *clix.Context, data any) error {
	return WriteWith(ctx, data, clix.FormatOptions{})
}

// WriteWith is Write with options for text output, such as the columns of
// a table (see clix.FormatOptions). The flags choose opts.Format.
func WriteWith(ctx *clix.Context, data any, opts clix.FormatOptions) error {
	if ctx == nil || ctx.App == nil {
		return errors.New("format.Write: no running command")
	}
	app := ctx.App
	out := app.Out
	if out == nil {
		out = os.Stdout
	}
	s, err := readSettings(app)
	if err != nil {
		return err
	}
	if err := s.check(); err != nil {
		return err
	}
	ls, err := readListSettings(ctx)
	if err != nil {
		return err
	}
	if ls.active() {
		if data, err = applyList(data, ls); err != nil {
			return err
		}
	}
	if len(s.jsonFlagNames) == 0 && s.rowTemplate == nil {
		if s.projection != nil {
			return writeProjection(out, data, s.projection, opts)
		}
		opts.Format = OutputFormat(app)
		return clix.FormatDataWith(out, data, opts)
	}

	var raw bytes.Buffer
	if err := clix.FormatData(&raw, data, clix.FormatJSON); err != nil {
		return err
	}
	doc := raw.Bytes()
	if s.json {
		available, err := availableFields(doc, data)
		if err != nil {
			return err
		}
		if len(s.fields) == 0 {
			return fieldsError("", available)
		}
		for _, f := range s.fields {
			if !fieldKnown(f, available) {
				return fieldsError(f, available)
			}
		}
		if doc, err = selectFields(doc, s.fields); err != nil {
			return err
		}
	}

	switch {
	case s.jq != "":
		q, err := jq.Parse(s.jq)
		if err != nil {
			return clix.UsageErrorf("invalid value for jq: %w", err)
		}
		var str func(string) string
		if escapeData(out, opts) {
			str = clix.EscapeControlChars
		}
		if err := q.WriteStrings(out, doc, str); err != nil {
			return fmt.Errorf("--jq: %w", err)
		}
		return nil
	case s.template != "":
		return executeTemplate(out, s.template, doc, escapeData(out, opts))
	case s.rowTemplate != nil && !s.json:
		return s.rowTemplate.write(out, doc, escapeData(out, opts))
	default:
		_, err := out.Write(doc)
		return err
	}
}

// settings are the output flags of one Run.
type settings struct {
	projection    *projection.Format // a gcloud format expression in --format (Extension.Projections)
	rowTemplate   *rowTemplate
	format        string // --format as given, in lower case ("" when unset)
	formatText    string // --format as given
	formatOnLine  bool   // --format was given on the command line
	json          bool   // --json was given
	fields        []string
	jq, template  string
	jsonFlagNames []string // the flags among --json, --jq and --template that were given
}

// UsesRowTemplate reports whether Write will render a --format row template.
// A command can use it to supply JSON fields normally omitted at their default
// values. Read it after flag parsing, from the running command.
func UsesRowTemplate(app *clix.App) bool {
	s, err := readSettings(app)
	return err == nil && s.rowTemplate != nil && len(s.jsonFlagNames) == 0
}

func readSettings(app *clix.App) (settings, error) {
	var s settings
	if v, ok := ours[*formatValue](app, "format"); ok {
		s.format, s.formatText = strings.ToLower(v.s), v.s
		s.formatOnLine = app.Flags().Changed("format")
		f, rows, err := v.parse(v.s)
		if err != nil {
			return s, clix.UsageErrorf("invalid value for format: %w", err)
		}
		s.rowTemplate = rows
		if f != nil {
			if (f.Name == clix.FormatJSON || f.Name == clix.FormatYAML) && !f.HasProjection {
				s.format = f.Name // " json " and the like: the format itself
			} else {
				s.projection = f
			}
		}
	}
	if v, ok := ours[*fieldsValue](app, "json"); ok && v.set {
		s.json, s.fields = true, v.fields
		s.jsonFlagNames = append(s.jsonFlagNames, "--json")
	}
	if v, ok := ours[*exprValue](app, "jq"); ok && v.s != "" {
		s.jq = v.s
		s.jsonFlagNames = append(s.jsonFlagNames, "--jq")
	}
	if v, ok := ours[*templateValue](app, "template"); ok && v.s != "" {
		s.template = v.s
		s.jsonFlagNames = append(s.jsonFlagNames, "--template")
	}
	return s, nil
}

// check reports flags that cannot be combined.
func (s settings) check() error {
	if s.jq != "" && s.template != "" {
		return clix.UsageErrorf("--jq and --template cannot be used together")
	}
	if len(s.jsonFlagNames) > 0 && s.formatOnLine && s.format != "" && (s.format != clix.FormatJSON || s.projection != nil) {
		format := s.format
		if s.projection != nil || s.rowTemplate != nil {
			format = s.formatText
		}
		return clix.UsageErrorf("%s works on JSON output and cannot be used with --format %s", strings.Join(s.jsonFlagNames, " and "), format)
	}
	return nil
}

// fieldsError is the usage error for --json without fields (field == "")
// or with a field the data does not have.
func fieldsError(field string, available []string) error {
	var b strings.Builder
	if field == "" {
		b.WriteString("specify one or more comma-separated fields for --json")
	} else {
		fmt.Fprintf(&b, "unknown field for --json: %q", field)
	}
	if len(available) == 0 {
		b.WriteString("; this output has no fields")
		return clix.UsageErrorf("%s", b.String())
	}
	if field == "" {
		b.WriteString(":")
	} else {
		b.WriteString("\navailable fields:")
	}
	for _, f := range available {
		b.WriteString("\n  ")
		b.WriteString(f)
	}
	return clix.UsageErrorf("%s", b.String())
}

// escapeData reports whether --jq and --template escape the control
// characters of the data they print: on a terminal, which a person reads,
// unless opts.RawControlChars. In a pipe or a file the output is for a
// program, which needs the exact text, as "jq -r" prints it.
func escapeData(out io.Writer, opts clix.FormatOptions) bool {
	return !opts.RawControlChars && tty.IsTerminal(out)
}
