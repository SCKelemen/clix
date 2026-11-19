package format

import (
	"fmt"
	"strings"

	"github.com/SCKelemen/clix/v2"
)

// Extension registers a global --format / -f flag on the app root.
// Import this extension to opt in to output format selection; without it
// the flag is not present and callers should default to clix.FormatText.
//
// The flag accepts json, yaml and text, in any case, and defaults to text
// (as does an empty value).
// Any other value, given on the command line, in the <PREFIX>_FORMAT
// environment variable or under the "format" config key, is a usage error
// that lists the valid formats.
//
// The app's own flags win over the extension's: when the app defines -f
// (for example, for --force), the extension's flag is --format only, and
// an app flag named --format replaces the extension's. App.Validate
// reports both. Adding the extension twice registers the flag once.
//
// Example:
//
//	app := clix.NewApp("myapp")
//	app.AddExtension(format.Extension{})
//	// Now --format / -f is available globally
type Extension struct{}

// formats lists the values the --format flag accepts.
var formats = []string{clix.FormatJSON, clix.FormatYAML, clix.FormatText}

// Extend implements clix.Extension.
func (Extension) Extend(app *clix.App) error {
	if f := app.Flags().Lookup("format"); f != nil {
		if _, ours := f.Value.(*formatValue); ours {
			return nil // already added
		}
	}
	app.Flags().Var(clix.VarOptions{
		FlagOptions: clix.FlagOptions{
			Name:     "format",
			Short:    "f",
			Usage:    "Output format (json, yaml, text)",
			Validate: validateFormat,
		},
		Default: clix.FormatText,
		Value:   &formatValue{},
	})
	// Shell completion offers the formats, not file names. When the app has
	// its own --format, that flag keeps its own completion.
	if f := app.Flags().Lookup("format"); f != nil {
		if _, ours := f.Value.(*formatValue); ours {
			app.Flags().SetCompletion("format", clix.CompleteValues(formats...))
		}
	}
	return nil
}

// formatValue holds the --format flag's value. Its type marks the flag as
// registered by this extension.
type formatValue struct{ s string }

func (v *formatValue) Set(s string) error {
	v.s = s
	return nil
}

func (v *formatValue) String() string { return v.s }

// Reset lets one App run more than once: each Run starts from the
// default again (see clix.FlagSet.Var).
func (v *formatValue) Reset() { v.s = "" }

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

// OutputFormat reads the --format flag from the app and validates it.
// Returns clix.FormatText if the flag is absent or invalid.
func OutputFormat(app *clix.App) string {
	flags := app.Flags()
	if flags == nil {
		return clix.FormatText
	}
	if v, ok := flags.String("format"); ok && v != "" {
		f := strings.ToLower(v)
		switch f {
		case clix.FormatJSON, clix.FormatYAML, clix.FormatText:
			return f
		}
	}
	return clix.FormatText
}
