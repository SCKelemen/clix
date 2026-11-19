package clix

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"io"
	"time"
)

// jsonOutputOptions configure JSON output (see FormatData). EscapeForHTML
// is off, the encoding/json/v2 default: &, < and > are written as is, since
// the escapes only matter when JSON is embedded in HTML and they make URLs
// hard to read.
var jsonOutputOptions = json.JoinOptions(
	json.Deterministic(true),
	jsontext.WithIndent("  "),
	jsontext.AllowInvalidUTF8(true),
	json.WithMarshalers(json.JoinMarshalers(
		json.MarshalToFunc(func(enc *jsontext.Encoder, d time.Duration) error {
			return enc.WriteToken(jsontext.String(isoDuration(d)))
		}),
		json.MarshalToFunc(func(enc *jsontext.Encoder, t time.Time) error {
			return enc.WriteToken(jsontext.String(isoTime(t)))
		}),
	)),
)

// jsonTextOptions configure the conversion of structs and other values for
// text output, where durations keep their human form ("1m30s").
var jsonTextOptions = json.JoinOptions(
	jsontext.AllowInvalidUTF8(true),
	json.WithMarshalers(json.JoinMarshalers(
		json.MarshalToFunc(func(enc *jsontext.Encoder, d time.Duration) error {
			return enc.WriteToken(jsontext.String(d.String()))
		}),
		json.MarshalToFunc(func(enc *jsontext.Encoder, t time.Time) error {
			return enc.WriteToken(jsontext.String(isoTime(t)))
		}),
	)),
)

// formatJSON formats data as indented JSON with encoding/json/v2 (see
// FormatData). Nothing is written when data cannot be encoded.
func formatJSON(w io.Writer, data interface{}) error {
	out, err := json.Marshal(data, jsonOutputOptions)
	if err != nil {
		return err
	}
	_, err = w.Write(append(out, '\n'))
	return err
}

// marshalForText encodes v as JSON for text output to convert.
func marshalForText(v interface{}) ([]byte, error) {
	return json.Marshal(v, jsonTextOptions)
}

// isoTime formats t in RFC 3339, the profile of ISO 8601 for timestamps,
// with fractional seconds only when they are non-zero.
// https://datatracker.ietf.org/doc/html/rfc3339#section-5.6
//
// TODO: switch to iso8601.FormatTime once
// github.com/SCKelemen/clix/v2/iso8601 lands.
func isoTime(t time.Time) string {
	return t.Format(time.RFC3339Nano)
}
