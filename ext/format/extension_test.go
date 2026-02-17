package format_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
)

func newAppWithFormat() *clix.App {
	app := clix.NewApp("test")
	app.AddExtension(format.Extension{})
	// Apply extensions eagerly so flags are registered before Parse.
	app.ApplyExtensions()
	return app
}

func TestOutputFormat(t *testing.T) {
	t.Run("default format is text", func(t *testing.T) {
		app := newAppWithFormat()
		if format.OutputFormat(app) != "text" {
			t.Errorf("expected default format 'text', got %q", format.OutputFormat(app))
		}
	})

	t.Run("format can be set to json", func(t *testing.T) {
		app := newAppWithFormat()
		app.Flags().Parse([]string{"--format=json"})

		if f := format.OutputFormat(app); f != "json" {
			t.Errorf("expected format 'json', got %q", f)
		}
	})

	t.Run("format can be set to yaml", func(t *testing.T) {
		app := newAppWithFormat()
		app.Flags().Parse([]string{"--format=yaml"})

		if f := format.OutputFormat(app); f != "yaml" {
			t.Errorf("expected format 'yaml', got %q", f)
		}
	})

	t.Run("invalid format defaults to text", func(t *testing.T) {
		app := newAppWithFormat()
		app.Flags().Parse([]string{"--format=invalid"})

		if f := format.OutputFormat(app); f != "text" {
			t.Errorf("expected invalid format to default to 'text', got %q", f)
		}
	})

	t.Run("format is case insensitive", func(t *testing.T) {
		app := newAppWithFormat()
		app.Flags().Parse([]string{"--format=JSON"})

		if f := format.OutputFormat(app); f != "json" {
			t.Errorf("expected format to be case insensitive 'json', got %q", f)
		}
	})

	t.Run("no extension means text", func(t *testing.T) {
		app := clix.NewApp("test")
		if f := format.OutputFormat(app); f != "text" {
			t.Errorf("expected 'text' without extension, got %q", f)
		}
	})
}

func TestFormatOutputViaExtension(t *testing.T) {
	app := newAppWithFormat()
	app.Out = &bytes.Buffer{}

	t.Run("format map as json", func(t *testing.T) {
		app.Flags().Parse([]string{"--format=json"})
		data := map[string]interface{}{
			"name":  "test",
			"value": 42,
		}

		f := format.OutputFormat(app)
		if err := clix.FormatData(app.Out, data, f); err != nil {
			t.Fatalf("FormatData failed: %v", err)
		}

		output := app.Out.(*bytes.Buffer).String()
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatalf("output is not valid JSON: %v, output: %s", err, output)
		}

		if result["name"] != "test" || result["value"] != float64(42) {
			t.Errorf("unexpected JSON output: %v", result)
		}

		app.Out.(*bytes.Buffer).Reset()
	})

	t.Run("format map as text", func(t *testing.T) {
		app.Flags().Parse([]string{"--format=text"})
		data := map[string]string{
			"name":  "test",
			"value": "42",
		}

		f := format.OutputFormat(app)
		if err := clix.FormatData(app.Out, data, f); err != nil {
			t.Fatalf("FormatData failed: %v", err)
		}

		output := app.Out.(*bytes.Buffer).String()
		if !strings.Contains(output, "name =") || !strings.Contains(output, "value =") {
			t.Errorf("output should contain text format, got: %s", output)
		}

		app.Out.(*bytes.Buffer).Reset()
	})
}
