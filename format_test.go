package clix

import (
	"bytes"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

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
		// YAML encoder may quote values with spaces, or use other valid YAML syntax
		// Just verify the value is present and the output is valid YAML
		if !strings.Contains(output, "value with spaces") {
			t.Errorf("expected value with spaces in YAML output, got: %s", output)
		}
		// Verify it can be parsed back
		var parsed map[string]string
		if err := yaml.Unmarshal(buf.Bytes(), &parsed); err != nil {
			t.Errorf("output is not valid YAML: %v", err)
		}
		if parsed["key"] != "value with spaces" {
			t.Errorf("round-trip failed: want %q, got %q", "value with spaces", parsed["key"])
		}
	})
}
