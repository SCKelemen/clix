package clix

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type textUser struct {
	UserName string `json:"user_name"`
	Age      int    `json:"age,omitempty"`
}

// textLevel has a MarshalJSON method, which text output must honor.
// (Named strings, numbers and booleans print with %v, as they always did.)
type textLevel struct{ n int }

func (l textLevel) MarshalJSON() ([]byte, error) { return json.Marshal([]string{"low", "high"}[l.n]) }

// textStringer prints through String, inside maps and lists too.
type textStringer struct{ id int }

func (s textStringer) String() string { return "stringer-" + string(rune('0'+s.id)) }

func formatTextString(t *testing.T, data interface{}) string {
	t.Helper()
	var b bytes.Buffer
	if err := FormatData(&b, data, FormatText); err != nil {
		t.Fatalf("FormatData(%#v): %v", data, err)
	}
	return b.String()
}

// Structs, slices of structs, maps of other value types, nested maps and
// nil printed Go syntax ("[{", "map[", "<nil>").
func TestFormatTextIsReadable(t *testing.T) {
	when := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, tc := range []struct {
		name string
		data interface{}
		want string
	}{
		{"nil", nil, ""},
		{"nil pointer", (*textUser)(nil), ""},
		{"struct", textUser{UserName: "ann"}, "user_name = ann\n"},
		{"struct pointer", &textUser{UserName: "ann", Age: 7}, "age = 7\nuser_name = ann\n"},
		{"slice of structs", []textUser{{UserName: "ann"}, {UserName: "bob", Age: 3}}, "user_name = ann\n\nage = 3\nuser_name = bob\n"},
		{"map[string]int", map[string]int{"b": 2, "a": 1}, "a = 1\nb = 2\n"},
		{"nested map", map[string]interface{}{"go": map[string]string{"os": "darwin", "arch": "arm64"}, "name": "x"}, "go.arch = arm64\ngo.os = darwin\nname = x\n"},
		{"deeply nested", map[string]interface{}{"a": map[string]interface{}{"b": map[string]int{"c": 1}}}, "a.b.c = 1\n"},
		{"empty nested map", map[string]interface{}{"a": map[string]string{}, "b": "x"}, "a = \nb = x\n"},
		{"list of values in a map", map[string]interface{}{"tags": []string{"a", "b"}, "n": []int{1, 2}}, "n = 1, 2\ntags = a, b\n"},
		{"list of objects in a map", map[string]interface{}{"items": []textUser{{UserName: "a"}, {UserName: "b"}}}, "items.0.user_name = a\nitems.1.user_name = b\n"},
		{"slice of ints", []int{1, 2, 3}, "1\n2\n3\n"},
		{"slice of maps", []map[string]string{{"a": "1"}, {"b": "2"}}, "a = 1\n\nb = 2\n"},
		{"mixed list", []interface{}{"x", map[string]int{"a": 1}}, "x\n\na = 1\n"},
		{"MarshalJSON", map[string]interface{}{"level": textLevel{1}, "users": []textUser{{UserName: "ann"}}}, "level = high\nusers.0.user_name = ann\n"},
		{"Stringer beside a nested map", map[string]interface{}{"id": textStringer{1}, "at": when, "m": map[string]int{"x": 1}}, "at = 2026-01-02 03:04:05 +0000 UTC\nid = stringer-1\nm.x = 1\n"},
		{"large integer", map[string]interface{}{"n": map[string]uint64{"max": 18446744073709551615}}, "n.max = 18446744073709551615\n"},
		{"float", map[string]float64{"pi": 3.25}, "pi = 3.25\n"},
		{"json.RawMessage", json.RawMessage(`{"a":{"b":[1,2]}}`), "a.b = 1, 2\n"},
	} {
		got := formatTextString(t, tc.data)
		if got != tc.want {
			t.Errorf("%s: text output %q, want %q", tc.name, got, tc.want)
		}
		if strings.Contains(got, "map[") || strings.Contains(got, "<nil>") || strings.Contains(got, "[{") {
			t.Errorf("%s: text output is Go syntax: %q", tc.name, got)
		}
	}
}

// The types text output always handled print exactly as before.
func TestFormatTextKeepsExistingOutput(t *testing.T) {
	for _, tc := range []struct {
		name string
		data interface{}
		want string
	}{
		{"map[string]string", map[string]string{"b": "2", "a": "x y"}, "a = x y\nb = 2\n"},
		{"map[string]interface{} of scalars", map[string]interface{}{"s": "v", "i": 3, "f": 1.5, "b": true, "n": nil, "u": uint8(7), "d": time.Second, "id": textStringer{2}}, "b = true\nd = 1s\nf = 1.5\ni = 3\nid = stringer-2\nn = \ns = v\nu = 7\n"},
		{"empty map", map[string]interface{}{}, ""},
		{"[]string", []string{"a", "b"}, "a\nb\n"},
		{"[]interface{} of scalars", []interface{}{"a", 1, nil, 2.5, textStringer{3}}, "a\n1\n\n2.5\nstringer-3\n"},
		{"string", "hello", "hello\n"},
		{"int", 42, "42\n"},
		{"float", 1e21, "1e+21\n"},
		{"Stringer", textStringer{4}, "stringer-4\n"},
		{"Stringer struct", time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), "2026-01-02 03:04:05 +0000 UTC\n"},
		{"complex", complex(1, 2), "(1+2i)\n"},
	} {
		if got := formatTextString(t, tc.data); got != tc.want {
			t.Errorf("%s: text output %q, want %q", tc.name, got, tc.want)
		}
	}
}

// A value encoding/json cannot convert still prints with %v.
func TestFormatTextFallsBackToFmt(t *testing.T) {
	ch := make(chan int)
	if got := formatTextString(t, ch); !strings.HasPrefix(got, "0x") {
		t.Errorf("text output of a channel: %q", got)
	}
	if got := formatTextString(t, map[string]interface{}{"ch": ch, "a": map[string]int{"x": 1}}); !strings.HasPrefix(got, "a.x = 1\nch = 0x") {
		t.Errorf("text output of a map holding a channel: %q", got)
	}
}
