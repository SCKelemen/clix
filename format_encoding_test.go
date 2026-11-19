package clix

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// JSON output escaped &, < and > as \u0026, \u003c and \u003e, mangling URLs.
func TestFormatJSONKeepsHTMLCharacters(t *testing.T) {
	data := map[string]string{"url": "https://x.test/?a=1&b=2", "tag": "<b>"}
	var b bytes.Buffer
	if err := FormatData(&b, data, FormatJSON); err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"tag\": \"<b>\",\n  \"url\": \"https://x.test/?a=1&b=2\"\n}\n"
	if b.String() != want {
		t.Errorf("json output %q, want %q", b.String(), want)
	}
	var back map[string]string
	if err := json.Unmarshal(b.Bytes(), &back); err != nil || back["url"] != data["url"] || back["tag"] != data["tag"] {
		t.Errorf("json output does not decode to the input: %v %v", back, err)
	}
}

func formatYAMLString(t *testing.T, data interface{}) string {
	t.Helper()
	var b bytes.Buffer
	if err := FormatData(&b, data, FormatYAML); err != nil {
		t.Fatalf("FormatData(%#v, yaml): %v", data, err)
	}
	return b.String()
}

// YAML of a json.RawMessage was a list of byte values.
func TestFormatYAMLDecodesRawJSON(t *testing.T) {
	raw := json.RawMessage(`{"a":1,"b":[true,"x"],"c":1.5,"big":18446744073709551615}`)
	for _, tc := range []struct {
		name string
		data interface{}
		want string
	}{
		{"top level", raw, "a: 1\nb:\n  - true\n  - x\nbig: 18446744073709551615\nc: 1.5\n"},
		{"in a map", map[string]interface{}{"spec": json.RawMessage(`{"a":1}`), "name": "x"}, "name: x\nspec:\n  a: 1\n"},
		{"map of raw", map[string]json.RawMessage{"spec": json.RawMessage(`"s"`)}, "spec: s\n"},
		{"in a slice", []json.RawMessage{json.RawMessage(`1`), json.RawMessage(`{"k":"v"}`)}, "- 1\n- k: v\n"},
		{"pointer", map[string]interface{}{"p": &raw}, "p:\n  a: 1\n  b:\n    - true\n    - x\n  big: 18446744073709551615\n  c: 1.5\n"},
		{"null", map[string]interface{}{"unset": json.RawMessage(nil), "null": json.RawMessage(`null`)}, "\"null\": null\nunset: null\n"},
		{"nested slice in map", map[string]interface{}{"list": []interface{}{json.RawMessage(`[1,2]`), "x"}}, "list:\n  - - 1\n    - 2\n  - x\n"},
	} {
		got := formatYAMLString(t, tc.data)
		if got != tc.want {
			t.Errorf("%s: yaml %q, want %q", tc.name, got, tc.want)
		}
		if strings.Contains(got, "- 123") {
			t.Errorf("%s: json.RawMessage rendered as bytes:\n%s", tc.name, got)
		}
		var v interface{}
		if err := yaml.Unmarshal([]byte(got), &v); err != nil {
			t.Errorf("%s: output is not YAML: %v", tc.name, err)
		}
	}

	var b bytes.Buffer
	if err := FormatData(&b, map[string]interface{}{"bad": json.RawMessage(`{`)}, FormatYAML); err == nil {
		t.Errorf("invalid json.RawMessage: no error, output %q", b.String())
	}
}

// Data without a RawMessage is encoded as given, so types with their own
// MarshalYAML and YAML key naming are unchanged.
func TestFormatYAMLLeavesOtherDataAlone(t *testing.T) {
	type user struct {
		UserName string `json:"user_name"`
	}
	data := map[string]interface{}{"users": []user{{UserName: "ann"}}, "count": 1}
	want := "count: 1\nusers:\n  - username: ann\n"
	if got := formatYAMLString(t, data); got != want {
		t.Errorf("yaml %q, want %q", got, want)
	}
	// v2 keeps YAML's own key naming for structs (field names in lower
	// case; json tags are ignored), since scripts may parse it. Following
	// json tags in YAML is a v3 change.
	if got := formatYAMLString(t, user{UserName: "ann"}); got != "username: ann\n" {
		t.Errorf("yaml of a struct %q changed", got)
	}
}
