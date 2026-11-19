//go:build go1.27

package clix

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func formatJSONString(t *testing.T, data interface{}) string {
	t.Helper()
	var b bytes.Buffer
	if err := FormatData(&b, data, FormatJSON); err != nil {
		t.Fatalf("FormatData(%#v, json): %v", data, err)
	}
	return b.String()
}

type jsonGoldenItem struct {
	Name    string            `json:"name"`
	Count   int               `json:"count,omitempty"`
	Skipped int               `json:"skipped,omitzero"`
	Note    string            `json:"note,omitempty"`
	Hidden  string            `json:"-"`
	Labels  map[string]string `json:"labels"`
	Tags    []string          `json:"tags"`
	Timeout time.Duration     `json:"timeout"`
	At      time.Time         `json:"at"`
	Raw     []byte            `json:"raw"`
}

// Golden JSON output of FormatData, encoded with encoding/json/v2.
func TestFormatJSONGolden(t *testing.T) {
	at := time.Date(2026, 11, 19, 8, 30, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		data interface{}
		want string
	}{
		{"map keys are sorted", map[string]interface{}{"zeta": 1, "alpha": map[string]int{"d": 4, "c": 3}, "mid": true},
			"{\n  \"alpha\": {\n    \"c\": 3,\n    \"d\": 4\n  },\n  \"mid\": true,\n  \"zeta\": 1\n}\n"},
		{"integer keys sort as strings", map[int]string{10: "a", 2: "b"}, "{\n  \"10\": \"a\",\n  \"2\": \"b\"\n}\n"},
		{"struct with tags", jsonGoldenItem{Name: "a", Labels: map[string]string{"b": "2", "a": "1"}, Tags: []string{"x"}, Timeout: 90 * time.Second, At: at, Raw: []byte("hi")},
			"{\n  \"name\": \"a\",\n  \"count\": 0,\n  \"labels\": {\n    \"a\": \"1\",\n    \"b\": \"2\"\n  },\n  \"tags\": [\n    \"x\"\n  ],\n  \"timeout\": \"PT1M30S\",\n  \"at\": \"2026-11-19T08:30:00Z\",\n  \"raw\": \"aGk=\"\n}\n"},
		{"nil and empty slices and maps", map[string]interface{}{
			"nilSlice": []int(nil), "emptySlice": []int{}, "nilMap": map[string]int(nil), "emptyMap": map[string]int{},
			"nilBytes": []byte(nil), "nilPointer": (*int)(nil), "nil": nil,
		}, "{\n  \"emptyMap\": {},\n  \"emptySlice\": [],\n  \"nil\": null,\n  \"nilBytes\": \"\",\n  \"nilMap\": {},\n  \"nilPointer\": null,\n  \"nilSlice\": []\n}\n"},
		{"durations", []time.Duration{0, 90 * time.Second, 1500 * time.Millisecond, -2 * time.Hour, 26 * time.Hour},
			"[\n  \"PT0S\",\n  \"PT1M30S\",\n  \"PT1.5S\",\n  \"-PT2H\",\n  \"PT26H\"\n]\n"},
		{"duration pointer and map value", map[string]interface{}{"p": ptrTo(time.Minute), "d": time.Millisecond},
			"{\n  \"d\": \"PT0.001S\",\n  \"p\": \"PT1M\"\n}\n"},
		{"times", []time.Time{at, at.Add(500 * time.Millisecond), at.In(time.FixedZone("CEST", 2*3600)), {}},
			"[\n  \"2026-11-19T08:30:00Z\",\n  \"2026-11-19T08:30:00.5Z\",\n  \"2026-11-19T10:30:00+02:00\",\n  \"0001-01-01T00:00:00Z\"\n]\n"},
		{"bytes", map[string][]byte{"b": {0, 1, 2, 255}}, "{\n  \"b\": \"AAEC/w==\"\n}\n"},
		{"nested values", map[string]interface{}{"a": []interface{}{map[string]interface{}{"b": []int{1}}, "c"}},
			"{\n  \"a\": [\n    {\n      \"b\": [\n        1\n      ]\n    },\n    \"c\"\n  ]\n}\n"},
		{"invalid UTF-8 is replaced", map[string]string{"k\xff": "bad\xfe"}, "{\n  \"k\ufffd\": \"bad\ufffd\"\n}\n"},
		{"HTML and JS characters are not escaped", map[string]string{"s": "<a href=\"x?a=1&b=2\">\u2028"}, "{\n  \"s\": \"<a href=\\\"x?a=1&b=2\\\">\u2028\"\n}\n"},
		{"raw JSON keeps its order", json.RawMessage(`{"b":1,"a":[true]}`), "{\n  \"b\": 1,\n  \"a\": [\n    true\n  ]\n}\n"},
		{"scalars", []interface{}{"s", 1.5, int64(math.MaxInt64), uint64(math.MaxUint64), false, nil}, "[\n  \"s\",\n  1.5,\n  9223372036854775807,\n  18446744073709551615,\n  false,\n  null\n]\n"},
		{"nil", nil, "null\n"},
	} {
		if got := formatJSONString(t, tc.data); got != tc.want {
			t.Errorf("%s:\n%s\nwant:\n%s", tc.name, got, tc.want)
		}
	}
}

func ptrTo[T any](v T) *T { return &v }

// A value that cannot be encoded is an error, and nothing is written.
func TestFormatJSONUnsupportedValue(t *testing.T) {
	var b bytes.Buffer
	if err := FormatData(&b, map[string]interface{}{"ok": 1, "ch": make(chan int)}, FormatJSON); err == nil {
		t.Fatal("encoding a channel succeeded")
	}
	if b.Len() != 0 {
		t.Errorf("partial output written: %q", b.String())
	}
}

func TestISODuration(t *testing.T) {
	for d, want := range map[time.Duration]string{
		0:                            "PT0S",
		90 * time.Second:             "PT1M30S",
		1500 * time.Millisecond:      "PT1.5S",
		-2 * time.Hour:               "-PT2H",
		26 * time.Hour:               "PT26H",
		time.Hour + time.Second:      "PT1H1S",
		time.Nanosecond:              "PT0.000000001S",
		-time.Millisecond:            "-PT0.001S",
		time.Duration(math.MaxInt64): "PT2562047H47M16.854775807S",
		time.Duration(math.MinInt64): "-PT2562047H47M16.854775808S",
		2*time.Hour + 3*time.Minute + 40*time.Microsecond: "PT2H3M0.00004S",
	} {
		if got := isoDuration(d); got != want {
			t.Errorf("isoDuration(%v) = %q, want %q", d, got, want)
		}
	}
}

// Text output keeps durations human and times in RFC 3339.
func TestFormatTextDurationsAndTimes(t *testing.T) {
	type job struct {
		Timeout time.Duration `json:"timeout"`
		At      time.Time     `json:"at"`
	}
	data := job{Timeout: 90 * time.Second, At: time.Date(2026, 11, 19, 8, 30, 0, 500, time.UTC)}
	var b bytes.Buffer
	if err := FormatData(&b, data, FormatText); err != nil {
		t.Fatal(err)
	}
	if want := "at = 2026-11-19T08:30:00.0000005Z\ntimeout = 1m30s\n"; b.String() != want {
		t.Errorf("text output %q, want %q", b.String(), want)
	}
	// omitempty follows encoding/json/v2: only null, "", {} and [] are
	// left out, so a zero number stays; omitzero leaves it out.
	type counts struct {
		Empty int `json:"empty,omitempty"`
		Zero  int `json:"zero,omitzero"`
	}
	b.Reset()
	_ = FormatData(&b, counts{}, FormatText)
	if b.String() != "empty = 0\n" {
		t.Errorf("text output of omitempty/omitzero zeros: %q", b.String())
	}
	if !strings.Contains(formatJSONString(t, counts{}), `"empty": 0`) {
		t.Error("JSON omitted a zero int marked omitempty")
	}
}
