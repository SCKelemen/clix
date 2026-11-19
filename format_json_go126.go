//go:build !go1.27

package clix

import (
	"bytes"
	"encoding/json"
	"io"
)

// Before Go 1.27, whose encoding/json/v2 FormatData uses (format_json.go),
// JSON output falls back to encoding/json. Delete this file once go.mod
// requires Go 1.27.

// formatJSON formats data as indented JSON with encoding/json, without
// escaping &, < and >. Nothing is written when data cannot be encoded.
func formatJSON(w io.Writer, data interface{}) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

// marshalForText encodes v as JSON for text output to convert.
func marshalForText(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
