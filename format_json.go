package clix

import (
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"io"
	"time"

	"github.com/SCKelemen/clix/v2/iso8601"
)

// jsonV1Semantics makes encoding/json/v2 encode Go values the way
// encoding/json (v1) does, so values that v1 handled keep working: the
// legacy omitempty, ",string" on booleans, MarshalJSON methods with pointer
// receivers only on addressable values, structs without exported fields as
// {}, duplicate names in MarshalJSON output, [N]byte as arrays, and v1's
// rules for embedded fields and conflicting tags. The options after it
// change only the formatting and the few representations FormatData
// documents.
var jsonV1Semantics = jsonv1.DefaultOptionsV1()

// jsonOutputOptions configure JSON output (see FormatData). EscapeForHTML
// and EscapeForJS are off: &, <, > and U+2028/U+2029 are written as is,
// since the escapes only matter when JSON is embedded in HTML or
// JavaScript and they make URLs hard to read.
var jsonOutputOptions = json.JoinOptions(
	jsonV1Semantics,
	json.Deterministic(true),
	jsontext.WithIndent("  "),
	jsontext.EscapeForHTML(false),
	jsontext.EscapeForJS(false),
	jsontext.AllowInvalidUTF8(true),
	json.FormatNilSliceAsNull(false),
	json.FormatNilMapAsNull(false),
	json.WithMarshalers(json.JoinMarshalers(
		json.MarshalToFunc(func(enc *jsontext.Encoder, d time.Duration) error {
			return enc.WriteToken(jsontext.String(iso8601.FormatDuration(d)))
		}),
		json.MarshalToFunc(func(enc *jsontext.Encoder, t time.Time) error {
			return enc.WriteToken(jsontext.String(isoTime(t)))
		}),
	)),
)

// jsonTextOptions configure the conversion of structs and other values for
// text output, where durations keep their human form ("1m30s").
var jsonTextOptions = json.JoinOptions(
	jsonV1Semantics,
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
// It is not iso8601.FormatTime, which differs for an offset with a seconds
// part (local mean time in historical zone data): FormatTime writes such a
// time in UTC, while Format drops the offset's seconds. YAML output, which
// go.yaml.in/yaml/v3 formats with Format, would then disagree with JSON.
func isoTime(t time.Time) string {
	return t.Format(time.RFC3339Nano)
}
