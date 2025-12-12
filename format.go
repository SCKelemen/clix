package clix

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// FormatOutput formats data according to the app's output format setting.
// It supports "json", "yaml", and "text" formats.
// For maps/slices, it automatically formats them appropriately.
// For other types, it falls back to text formatting.
func (a *App) FormatOutput(data interface{}) error {
	format := a.OutputFormat()
	return FormatData(a.Out, data, format)
}

// FormatData formats data to the specified output writer using the given format.
func FormatData(w io.Writer, data interface{}, format string) error {
	switch strings.ToLower(format) {
	case "json":
		return formatJSON(w, data)
	case "yaml":
		return formatYAML(w, data)
	default:
		return formatText(w, data)
	}
}

// formatJSON formats data as JSON with indentation.
func formatJSON(w io.Writer, data interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// formatYAML formats data as YAML using yaml.v3.
func formatYAML(w io.Writer, data interface{}) error {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	defer encoder.Close()
	return encoder.Encode(data)
}

// formatText formats data as plain text (default).
func formatText(w io.Writer, data interface{}) error {
	switch v := data.(type) {
	case map[string]interface{}:
		return formatTextMap(w, v)
	case map[string]string:
		// Convert map[string]string to map[string]interface{}
		m := make(map[string]interface{})
		for k, val := range v {
			m[k] = val
		}
		return formatTextMap(w, m)
	case []interface{}:
		return formatTextList(w, v)
	case []string:
		// Convert []string to []interface{}
		list := make([]interface{}, len(v))
		for i, val := range v {
			list[i] = val
		}
		return formatTextList(w, list)
	default:
		fmt.Fprintf(w, "%v\n", v)
		return nil
	}
}

func formatTextMap(w io.Writer, m map[string]interface{}) error {
	// Sort keys for consistent output
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		fmt.Fprintf(w, "%s = %s\n", key, formatValue(m[key]))
	}
	return nil
}

func formatTextList(w io.Writer, list []interface{}) error {
	for _, item := range list {
		fmt.Fprintf(w, "%s\n", formatValue(item))
	}
	return nil
}

// formatValue converts a value to its string representation.
func formatValue(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", val)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val)
	case float32, float64:
		return fmt.Sprintf("%g", val)
	case bool:
		return fmt.Sprintf("%t", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}
