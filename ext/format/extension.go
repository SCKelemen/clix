package format

import (
	"fmt"
	"slices"
	"strings"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/internal/jq"
	"github.com/SCKelemen/clix/v2/internal/projection"
)

// Extension registers the output flags on the app root, modeled on the
// GitHub CLI (gh):
//
//   - --format / -f selects json, yaml or text (the default). FormatShort
//     chooses another short name, or none.
//   - --json [fields] prints JSON, restricted to the named fields when
//     some are given (see Write).
//   - --jq / -q <expression> filters the JSON output with a jq expression
//     (see the package documentation for the supported subset of jq).
//   - --template / -t <template> formats the JSON output with a Go
//     text/template (see the package documentation for the helpers).
//
// Import this extension to opt in; without it the flags are not present
// and callers should default to clix.FormatText. Commands print their
// result with Write, which applies all four flags; commands that call
// clix.FormatData with OutputFormat honor --format (and --json, as plain
// JSON) only.
//
// --format accepts json, yaml and text, in any case, and defaults to text
// (as does an empty value). Any other value, given on the command line,
// in the <PREFIX>_FORMAT environment variable or under the "format" config
// key, is a usage error that lists the valid formats.
//
// --json, --jq and --template are read from the command line only (they
// depend on the command), never from the environment or the config file
// (see clix.FlagOptions.CommandLineOnly). A --jq expression or a
// --template that does not parse is a usage error before the command
// runs.
//
// All four flags are Ambient (see clix.FlagOptions.Ambient): they choose
// how the result is printed, not what the command does, so "myapp --format
// json widgets create" still prompts for the command's missing required
// flags when the app is interactive.
//
// The app's own flags win over the extension's: when the app defines -f
// (for example, for --force), -q or -t, the extension's flag keeps only its
// long name, and an app flag named --format, --json, --jq or --template
// replaces the extension's (Write then ignores it). App.Validate reports
// replaced flags. Adding the extension twice registers the flags once.
//
// A command's own -f wins after the command name: in "myapp apply -f
// x.yaml", -f is apply's flag, while before the command name it is
// --format. App.Validate reports that, since the two meanings of -f can
// surprise users. To keep -f for the commands, set FormatShort to NoShort
// (as in gcloud, whose --format has no short name) or to another letter.
//
// That includes gcloud's -q for --quiet: clix.App.EnableQuiet registers
// --quiet without a short name, so -q stays --jq, as in gh. An app that
// calls app.EnableQuiet(clix.QuietOptions{Short: "q"}) itself owns -q,
// and --jq is then available by its long name only.
//
// With Projections set, --format also accepts gcloud's format
// expressions, NAME[ATTRIBUTES](PROJECTION), such as "table(name,
// zone:label=LOCATION)" or "value(name)" (see the package documentation
// for the supported subset). json, yaml and text, with nothing after
// them, keep printing exactly as without Projections.
//
// Example:
//
//	app := clix.NewApp("myapp")
//	app.AddExtension(format.Extension{})
//	// Now --format, --json, --jq and --template are available globally
type Extension struct {
	// Projections makes --format accept gcloud's format expressions:
	// table, csv, value, flattened, list, json and yaml with attributes
	// and projections, and none. Off by default, so apps that do not set
	// it are unchanged.
	Projections bool

	// FormatShort is the short name of --format: empty for the default,
	// "f", or NoShort to register --format by its long name only. A
	// letter that --jq (-q) or --template (-t) would use goes to
	// --format, and that flag keeps only its long name (unless the app
	// defines its own --format, which then has its own short name). As
	// with any flag, "h" takes -h from --help, and an app's flag with the
	// same letter keeps it (see clix.App.Validate).
	FormatShort string

	// RowTemplates accepts Docker-style Go templates in --format. Write
	// renders the template once per list item, or once for a single object.
	RowTemplates bool

	// MissingFields says what a row template does with a field its record
	// lacks. MissingError, the default, makes it a usage error, before any
	// row is printed. MissingZero prints it as nothing: a missing field,
	// or a field of a missing or null value, is nil in the template, so
	// it prints as an empty string, is false in an if and ranges over
	// nothing. The fields a Schema declares are never missing, since
	// Write fills them in (see Document).
	MissingFields Missing
}

// Missing is what a row template does with a field its record lacks
// (see Extension.MissingFields).
type Missing uint8

const (
	// MissingError makes a field the record lacks a usage error. It is
	// the default.
	MissingError Missing = iota
	// MissingZero prints a field the record lacks as nothing.
	MissingZero
)

// NoShort is the Extension.FormatShort that gives --format no short name.
const NoShort = "-"

// shorts returns the short names of --format, --jq and --template.
func (e Extension) shorts() (format, jq, template string) {
	format, jq, template = e.FormatShort, "q", "t"
	switch format {
	case "":
		format = "f"
	case NoShort:
		format = ""
	}
	if format == jq {
		jq = ""
	}
	if format == template {
		template = ""
	}
	return format, jq, template
}

// formats lists the values the --format flag accepts.
var formats = []string{clix.FormatJSON, clix.FormatYAML, clix.FormatText}

// Extend implements clix.Extension.
func (e Extension) Extend(app *clix.App) error {
	fs := app.Flags()
	formatShort, jqShort, templateShort := e.shorts()
	if _, ok := ours[*formatValue](app, "format"); !ok {
		usage, names := "Output format (json, yaml, text)", formats
		value := &formatValue{projections: e.Projections, rowTemplates: e.RowTemplates, missing: e.MissingFields}
		if e.Projections {
			usage = "Output format: json, yaml, text, or NAME[ATTRIBUTES](PROJECTION) with " + strings.Join(projection.Names, ", ")
			names = slices.Clone(formats)
			for _, n := range projection.Names {
				if !slices.Contains(names, n) {
					names = append(names, n)
				}
			}
		}
		if e.RowTemplates {
			usage += " or a row template such as '{{.name}}'"
		}
		fs.Var(clix.VarOptions{
			FlagOptions: clix.FlagOptions{
				Name:     "format",
				Short:    formatShort,
				Usage:    usage,
				Validate: value.validate,
				// An output setting, not an input: passing it does not stop
				// Run from prompting for missing required flags.
				Ambient: true,
			},
			Default: clix.FormatText,
			Value:   value,
		})
		// Shell completion offers the formats, not file names. When the app
		// has its own --format, that flag keeps its own completion.
		if _, ok := ours[*formatValue](app, "format"); ok {
			fs.SetCompletion("format", clix.CompleteValues(names...))
		}
	}
	// --jq and --template give their letter up only to this extension's
	// --format, not to an app's own.
	if _, ok := ours[*formatValue](app, "format"); !ok {
		jqShort, templateShort = "q", "t"
	}
	if _, ok := ours[*fieldsValue](app, "json"); !ok {
		fs.Var(clix.VarOptions{
			FlagOptions: clix.FlagOptions{
				Name:            "json",
				Usage:           "Output JSON with the specified fields (comma-separated)",
				Validate:        validateFields,
				Ambient:         true,
				CommandLineOnly: true,
			},
			Value: &fieldsValue{},
		})
		if _, ok := ours[*fieldsValue](app, "json"); ok {
			fs.SetCompletion("json", noCompletion)
		}
	}
	if _, ok := ours[*exprValue](app, "jq"); !ok {
		fs.Var(clix.VarOptions{
			FlagOptions: clix.FlagOptions{
				Name:            "jq",
				Short:           jqShort,
				Usage:           "Filter JSON output using a jq expression",
				Validate:        validateJQ,
				Ambient:         true,
				CommandLineOnly: true,
			},
			Value: &exprValue{},
		})
		if _, ok := ours[*exprValue](app, "jq"); ok {
			fs.SetCompletion("jq", noCompletion)
		}
	}
	if _, ok := ours[*templateValue](app, "template"); !ok {
		fs.Var(clix.VarOptions{
			FlagOptions: clix.FlagOptions{
				Name:            "template",
				Short:           templateShort,
				Usage:           "Format JSON output using a Go template",
				Validate:        validateTemplate,
				Ambient:         true,
				CommandLineOnly: true,
			},
			Value: &templateValue{},
		})
		if _, ok := ours[*templateValue](app, "template"); ok {
			fs.SetCompletion("template", noCompletion)
		}
	}
	return nil
}

// noCompletion offers nothing: the value is free text, not a file name.
func noCompletion(clix.CompletionRequest) []clix.Completion { return nil }

// ours returns the Value of the root flag name when this extension
// registered it and no app flag of that name replaced it.
func ours[V clix.Value](app *clix.App, name string) (V, bool) {
	var zero V
	if app == nil {
		return zero, false
	}
	f := app.Flags().Lookup(name)
	if f == nil {
		return zero, false
	}
	v, ok := f.Value.(V)
	return v, ok
}

// formatValue holds the --format flag's value. Its type marks the flag as
// registered by this extension.
type formatValue struct {
	s            string
	projections  bool // Extension.Projections: gcloud format expressions are accepted
	rowTemplates bool
	missing      Missing // Extension.MissingFields
}

func (v *formatValue) Set(s string) error {
	v.s = s
	return nil
}

func (v *formatValue) String() string { return v.s }

// Reset lets one App run more than once: each Run starts from the
// default again (see clix.FlagSet.Var).
func (v *formatValue) Reset() { v.s = "" }

// fieldsValue holds --json. Its value is optional (see
// clix.VarOptions.Value): "--json" alone, or followed by another flag,
// asks for the list of fields.
type fieldsValue struct {
	set    bool
	fields []string
	text   string
}

func (v *fieldsValue) Set(s string) error {
	v.set, v.text, v.fields = true, s, splitFields(s)
	return nil
}

func (v *fieldsValue) String() string { return v.text }

// OptionalValue makes "--json" without fields valid on the command line.
func (v *fieldsValue) OptionalValue() bool { return true }

func (v *fieldsValue) Reset() { *v = fieldsValue{} }

// exprValue holds --jq.
type exprValue struct{ s string }

func (v *exprValue) Set(s string) error {
	v.s = s
	return nil
}

func (v *exprValue) String() string { return v.s }
func (v *exprValue) Reset()         { v.s = "" }

// templateValue holds --template.
type templateValue struct{ s string }

func (v *templateValue) Set(s string) error {
	v.s = s
	return nil
}

func (v *templateValue) String() string { return v.s }
func (v *templateValue) Reset()         { v.s = "" }

// validateFormat accepts the formats in formats, ignoring case. An empty
// value means the default, text.
func validateFormat(value string) error {
	if value == "" {
		return nil
	}
	for _, f := range formats {
		if strings.EqualFold(value, f) {
			return nil
		}
	}
	return fmt.Errorf("unknown format %q (valid formats: %s)", value, strings.Join(formats, ", "))
}

// isPlainFormat reports whether value is one of the formats --format
// accepts without Projections (or empty, the default).
func isPlainFormat(value string) bool {
	return value == "" || validateFormat(value) == nil
}

func (v *formatValue) validate(value string) error {
	_, _, err := v.parse(value)
	return err
}

func (v *formatValue) parse(value string) (*projection.Format, *rowTemplate, error) {
	if isPlainFormat(value) {
		return nil, nil, nil
	}
	isTemplate := v.rowTemplates && strings.Contains(value, "{{")
	if v.projections {
		// A projection can have a quoted label or separator containing "{{".
		p, err := parseProjection(value)
		if err == nil || !isTemplate {
			return p, nil, err
		}
	}
	if isTemplate {
		t, err := parseRowTemplate(value, v.missing)
		return nil, t, err
	}
	return nil, nil, validateFormat(value)
}

// parseProjection parses a gcloud format expression, naming what clix
// does not support as --format's.
func parseProjection(value string) (*projection.Format, error) {
	f, err := projection.Parse(value)
	if err != nil {
		return nil, unsupportedIn(err, "--format")
	}
	return f, nil
}

// splitFields splits a --json value on commas, trimming spaces and
// dropping empty items.
func splitFields(s string) []string {
	var fields []string
	for _, f := range strings.Split(s, ",") {
		if f = strings.TrimSpace(f); f != "" {
			fields = append(fields, f)
		}
	}
	return fields
}

// validateFields checks the syntax of a --json value; which fields exist
// is known only once the command has its result (see Write).
func validateFields(value string) error {
	for _, f := range splitFields(value) {
		if strings.HasPrefix(f, ".") || strings.HasSuffix(f, ".") || strings.Contains(f, "..") {
			return fmt.Errorf("invalid field %q: a field is a name or a dotted path such as owner.login", f)
		}
	}
	return nil
}

// validateJQ parses a --jq expression, so a mistake is a usage error
// before the command runs.
func validateJQ(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("the expression is empty")
	}
	_, err := jq.Parse(value)
	return err
}

// validateTemplate parses a --template, so a mistake is a usage error
// before the command runs.
func validateTemplate(value string) error {
	if value == "" {
		return fmt.Errorf("the template is empty")
	}
	_, err := parseTemplate(value, &templateState{})
	return err
}

// OutputFormat reads the --format flag from the app and validates it.
// Returns clix.FormatText if the flag is absent or invalid, and
// clix.FormatJSON when --json was given. It does not apply --json's
// fields, --jq or --template; Write does.
func OutputFormat(app *clix.App) string {
	if v, ok := ours[*fieldsValue](app, "json"); ok && v.set {
		return clix.FormatJSON
	}
	if app == nil {
		return clix.FormatText
	}
	if v, ok := ours[*formatValue](app, "format"); ok && v.projections && !isPlainFormat(v.s) {
		// A gcloud format expression: json(...) and yaml(...) are JSON
		// and YAML, everything else is text for handlers that print
		// with clix.FormatData themselves (Write prints the expression).
		if f, err := projection.Parse(v.s); err == nil && (f.Name == clix.FormatJSON || f.Name == clix.FormatYAML) {
			return f.Name
		}
		return clix.FormatText
	}
	if v, ok := app.Flags().String("format"); ok && v != "" {
		f := strings.ToLower(v)
		switch f {
		case clix.FormatJSON, clix.FormatYAML, clix.FormatText:
			return f
		}
	}
	return clix.FormatText
}
