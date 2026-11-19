package format

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
)

type fieldsEmbedded struct {
	Inner  string `json:"inner"`
	Shadow string `json:"shadow"`
}

type fieldsAll struct {
	fieldsEmbedded
	Shadow   string            `json:"shadow"` // shallower, wins over the embedded one
	Text     string            `json:"text"`
	Raw      json.RawMessage   `json:"raw"`
	Num      json.Number       `json:"num"`
	Big      uint64            `json:"big"`
	F        float64           `json:"f"`
	D        time.Duration     `json:"d"`
	T        time.Time         `json:"t"`
	Map      map[string]any    `json:"map"`
	Bytes    []byte            `json:"bytes"`
	Ptr      *int              `json:"ptr"`
	Omit     string            `json:"omit,omitempty"`
	Dash     string            `json:"dash-name"`
	Untagged int               // encoded under its Go name
	Labels   map[string]string `json:"labels"`
	private  int
}

// Selecting every field reproduces clix.FormatData's JSON byte for byte:
// escapes, numbers, raw messages and nesting are copied as they are.
func TestSelectAllFieldsIsFormatData(t *testing.T) {
	one := 1
	values := []any{
		fieldsAll{
			fieldsEmbedded: fieldsEmbedded{Inner: "in", Shadow: "deep"},
			Shadow:         "top", Text: "ctl \x00\x01\x1f\x7f \"q\" \\ <&>    é 日本 \xff",
			Raw: json.RawMessage(`{"z":1,"a":[1.50,2e3,-0]}`), Num: "12345678901234567890123", Big: math.MaxUint64,
			F: 1e21, D: 90 * time.Second, T: time.Date(2026, 1, 2, 3, 4, 5, 6, time.UTC),
			Map: map[string]any{"b": []any{}, "a": map[string]any{}}, Bytes: []byte("hi"), Ptr: &one,
			Omit: "x", Dash: "dash", Untagged: 3, Labels: map[string]string{"app.kubernetes.io/name": "web"},
			private: 9,
		},
		[]fieldsAll{{}, {Text: "second", F: 0.1}},
	}
	all := []string{"inner", "shadow", "text", "raw", "num", "big", "f", "d", "t", "map", "bytes", "ptr", "omit", "dash-name", "Untagged", "labels"}
	for _, v := range values {
		var want bytes.Buffer
		if err := clix.FormatData(&want, v, clix.FormatJSON); err != nil {
			t.Fatal(err)
		}
		got, err := selectFields(want.Bytes(), all)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want.String() {
			t.Errorf("selectFields(all) differs from FormatData:\ngot:  %q\nwant: %q", got, want.String())
		}
		available, err := availableFields(want.Bytes(), v)
		if err != nil {
			t.Fatal(err)
		}
		wantFields := []string{"Untagged", "big", "bytes", "d", "dash-name", "f", "inner", "labels", "map", "num", "omit", "ptr", "raw", "shadow", "t", "text"}
		if !reflect.DeepEqual(available, wantFields) {
			t.Errorf("availableFields = %q, want %q", available, wantFields)
		}
	}
}

// A key containing dots is selected as a whole; a dotted path goes into
// nested objects.
func TestSelectDottedKeys(t *testing.T) {
	doc := []byte(`{"labels": {"app.kubernetes.io/name": "web", "tier": "db"}, "a.b": 1, "a": {"b": 2, "c": 3}}`)
	got, err := selectFields(doc, []string{"labels.app.kubernetes.io/name", "a.b"})
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"labels\": {\n    \"app.kubernetes.io/name\": \"web\"\n  },\n  \"a.b\": 1,\n  \"a\": {\n    \"b\": 2\n  }\n}\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if !fieldKnown("labels.anything", []string{"labels"}) || fieldKnown("label", []string{"labels"}) || !fieldKnown("a.b", []string{"a.b"}) {
		t.Error("fieldKnown")
	}
}

// structFields follows encoding/json's naming: ambiguous embedded fields
// at the same depth are dropped unless exactly one is tagged.
func TestStructFieldsAmbiguity(t *testing.T) {
	type A struct{ Name, Tagged string }
	type B struct {
		Name   string
		Tagged string `json:"Tagged"`
	}
	type both struct {
		A
		B
		Own string
	}
	if got, want := structFields(reflect.TypeOf([]*both{})), []string{"Own", "Tagged"}; !reflect.DeepEqual(sortedCopy(got), want) {
		t.Errorf("structFields = %q, want %q", got, want)
	}
	if got := structFields(reflect.TypeOf(time.Time{})); got != nil {
		t.Errorf("time.Time has fields %q", got)
	}
	if got := structFields(reflect.TypeOf(map[string]int{})); got != nil {
		t.Errorf("a map has fields %q", got)
	}
}

func sortedCopy(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return out
}
