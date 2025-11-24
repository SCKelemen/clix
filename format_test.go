package clix

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestOutputFormat(t *testing.T) {
	t.Run("default format is text", func(t *testing.T) {
		app := NewApp("test")
		if app.OutputFormat() != "text" {
			t.Errorf("expected default format 'text', got %q", app.OutputFormat())
		}
	})

	t.Run("format can be set to json", func(t *testing.T) {
		app := NewApp("test")
		// Use the root created by NewApp (it has the format flag)
		app.Root.Run = func(ctx *Context) error {
			return nil
		}

		// Parse the flag directly
		app.Flags().Parse([]string{"--format=json"})

		// After parsing, the flag should be set
		if format, _ := app.Flags().String("format"); format != "json" {
			t.Errorf("expected format flag to be 'json', got %q", format)
		}
	})

	t.Run("format can be set to yaml", func(t *testing.T) {
		app := NewApp("test")
		app.Flags().Parse([]string{"--format=yaml"})

		if app.OutputFormat() != "yaml" {
			t.Errorf("expected format 'yaml', got %q", app.OutputFormat())
		}
	})

	t.Run("invalid format defaults to text", func(t *testing.T) {
		app := NewApp("test")
		app.Flags().Parse([]string{"--format=invalid"})

		if app.OutputFormat() != "text" {
			t.Errorf("expected invalid format to default to 'text', got %q", app.OutputFormat())
		}
	})

	t.Run("format is case insensitive", func(t *testing.T) {
		app := NewApp("test")
		app.Flags().Parse([]string{"--format=JSON"})

		if app.OutputFormat() != "json" {
			t.Errorf("expected format to be case insensitive 'json', got %q", app.OutputFormat())
		}
	})
}

func TestFormatOutput(t *testing.T) {
	app := NewApp("test")
	app.Out = &bytes.Buffer{}

	t.Run("format map as json", func(t *testing.T) {
		app.Flags().Parse([]string{"--format=json"})
		data := map[string]interface{}{
			"name":  "test",
			"value": 42,
		}

		if err := app.FormatOutput(data); err != nil {
			t.Fatalf("FormatOutput failed: %v", err)
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

	t.Run("format map as yaml", func(t *testing.T) {
		app.Flags().Parse([]string{"--format=yaml"})
		data := map[string]string{
			"name":  "test",
			"value": "42",
		}

		if err := app.FormatOutput(data); err != nil {
			t.Fatalf("FormatOutput failed: %v", err)
		}

		output := app.Out.(*bytes.Buffer).String()
		if !strings.Contains(output, "name:") || !strings.Contains(output, "value:") {
			t.Errorf("output should contain YAML keys, got: %s", output)
		}

		app.Out.(*bytes.Buffer).Reset()
	})

	t.Run("format map as text", func(t *testing.T) {
		app.Flags().Parse([]string{"--format=text"})
		data := map[string]string{
			"name":  "test",
			"value": "42",
		}

		if err := app.FormatOutput(data); err != nil {
			t.Fatalf("FormatOutput failed: %v", err)
		}

		output := app.Out.(*bytes.Buffer).String()
		if !strings.Contains(output, "name =") || !strings.Contains(output, "value =") {
			t.Errorf("output should contain text format, got: %s", output)
		}

		app.Out.(*bytes.Buffer).Reset()
	})

	t.Run("format slice as json", func(t *testing.T) {
		app.Flags().Parse([]string{"--format=json"})
		data := []string{"item1", "item2", "item3"}

		if err := app.FormatOutput(data); err != nil {
			t.Fatalf("FormatOutput failed: %v", err)
		}

		output := app.Out.(*bytes.Buffer).String()
		var result []string
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatalf("output is not valid JSON: %v, output: %s", err, output)
		}

		if len(result) != 3 || result[0] != "item1" {
			t.Errorf("unexpected JSON output: %v", result)
		}

		app.Out.(*bytes.Buffer).Reset()
	})

	t.Run("format slice as yaml", func(t *testing.T) {
		app.Flags().Parse([]string{"--format=yaml"})
		data := []string{"item1", "item2"}

		if err := app.FormatOutput(data); err != nil {
			t.Fatalf("FormatOutput failed: %v", err)
		}

		output := app.Out.(*bytes.Buffer).String()
		if !strings.Contains(output, "- ") {
			t.Errorf("output should contain YAML list format, got: %s", output)
		}

		app.Out.(*bytes.Buffer).Reset()
	})

	t.Run("format slice as text", func(t *testing.T) {
		app.Flags().Parse([]string{"--format=text"})
		data := []string{"item1", "item2"}

		if err := app.FormatOutput(data); err != nil {
			t.Fatalf("FormatOutput failed: %v", err)
		}

		output := app.Out.(*bytes.Buffer).String()
		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) != 2 || lines[0] != "item1" || lines[1] != "item2" {
			t.Errorf("unexpected text output: %s", output)
		}

		app.Out.(*bytes.Buffer).Reset()
	})
}

func TestFormatData(t *testing.T) {
	t.Run("handles empty map", func(t *testing.T) {
		var buf bytes.Buffer
		data := map[string]interface{}{}

		if err := FormatData(&buf, data, "json"); err != nil {
			t.Fatalf("FormatData failed: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "{}") {
			t.Errorf("expected empty JSON object, got: %s", output)
		}
	})

	t.Run("handles nil values", func(t *testing.T) {
		var buf bytes.Buffer
		data := map[string]interface{}{
			"key": nil,
		}

		if err := FormatData(&buf, data, "text"); err != nil {
			t.Fatalf("FormatData failed: %v", err)
		}

		// Should handle nil gracefully
		output := buf.String()
		if !strings.Contains(output, "key =") {
			t.Errorf("expected key in output, got: %s", output)
		}
	})

	t.Run("quotes strings with spaces in yaml", func(t *testing.T) {
		var buf bytes.Buffer
		data := map[string]string{
			"key": "value with spaces",
		}

		if err := FormatData(&buf, data, "yaml"); err != nil {
			t.Fatalf("FormatData failed: %v", err)
		}

		output := buf.String()
		// YAML should quote values with spaces
		if !strings.Contains(output, `"value with spaces"`) {
			t.Errorf("expected quoted value in YAML, got: %s", output)
		}
	})
}
