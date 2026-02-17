package format

import (
	"strings"

	"github.com/SCKelemen/clix/v2"
)

// Extension registers a global --format / -f flag on the app root.
// Import this extension to opt in to output format selection; without it
// the flag is not present and callers should default to clix.FormatText.
//
// Example:
//
//	app := clix.NewApp("myapp")
//	app.AddExtension(format.Extension{})
//	// Now --format / -f is available globally
type Extension struct{}

// Extend implements clix.Extension.
func (Extension) Extend(app *clix.App) error {
	var format = clix.FormatText
	app.Flags().StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:  "format",
			Short: "f",
			Usage: "Output format (json, yaml, text)",
		},
		Default: clix.FormatText,
		Value:   &format,
	})
	return nil
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
