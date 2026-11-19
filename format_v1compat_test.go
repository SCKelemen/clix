package clix

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type compatStringBool struct {
	B bool `json:"b,string"`
}

type compatWithError struct {
	Err error `json:"err"`
}

type compatWithURL struct {
	U *url.URL `json:"u"`
}

type compatWithMutex struct {
	Mu sync.Mutex
	N  int `json:"n"`
}

type compatNoExported struct{ a int }

type compatInt int

type compatEmbedded struct {
	compatInt
	fmt.Stringer
	X int `json:"x"`
}

// taggedStruct builds a struct of int fields F0, F1, ... with the given
// json tags and values 1, 2, ... at run time, for tags that go vet or
// staticcheck reject in a literal (duplicate names, json:"-,").
func taggedStruct(tags ...string) interface{} {
	fields := make([]reflect.StructField, len(tags))
	for i, tag := range tags {
		fields[i] = reflect.StructField{Name: fmt.Sprintf("F%d", i), Type: reflect.TypeOf(0), Tag: reflect.StructTag(`json:"` + tag + `"`)}
	}
	v := reflect.New(reflect.StructOf(fields)).Elem()
	for i := range tags {
		v.Field(i).SetInt(int64(i + 1))
	}
	return v.Interface()
}

type compatDuplicateKeys struct{}

func (compatDuplicateKeys) MarshalJSON() ([]byte, error) { return []byte(`{"a":1,"a":2}`), nil }

type compatPointerMarshaler struct{}

func (*compatPointerMarshaler) MarshalJSON() ([]byte, error) { return []byte(`"called"`), nil }

type compatHolder struct {
	P compatPointerMarshaler `json:"p"`
}

// Values encoding/json (v1) encodes must keep encoding: encoding/json/v2
// on its own rejects or changes each of these.
func TestFormatJSONKeepsV1Semantics(t *testing.T) {
	u, _ := url.Parse("https://user@x.test/p?q=1")
	for _, tc := range []struct {
		name string
		data interface{}
		want string
	}{
		{"string option on a bool", compatStringBool{true}, "{\n  \"b\": \"true\"\n}\n"},
		{"error field", compatWithError{errors.New("boom")}, "{\n  \"err\": {}\n}\n"},
		{"url.URL", compatWithURL{u}, "{\n  \"u\": {\n    \"Scheme\": \"https\",\n    \"Opaque\": \"\",\n    \"User\": {},\n    \"Host\": \"x.test\",\n    \"Path\": \"/p\",\n    \"Fragment\": \"\",\n    \"RawQuery\": \"q=1\",\n    \"RawPath\": \"\",\n    \"RawFragment\": \"\",\n    \"ForceQuery\": false,\n    \"OmitHost\": false\n  }\n}\n"},
		{"sync.Mutex field", &compatWithMutex{N: 1}, "{\n  \"Mu\": {},\n  \"n\": 1\n}\n"},
		{"no exported fields", compatNoExported{1}, "{}\n"},
		{"embedded non-struct and interface", compatEmbedded{compatInt: 3, X: 1}, "{\n  \"Stringer\": null,\n  \"x\": 1\n}\n"},
		{"json:\"-,\" names a field -", taggedStruct("-,"), "{\n  \"-\": 1\n}\n"},
		{"duplicate tag names drop both", taggedStruct("x", "x", "c"), "{\n  \"c\": 3\n}\n"},
		{"MarshalJSON with duplicate keys", compatDuplicateKeys{}, "{\n  \"a\": 1,\n  \"a\": 2\n}\n"},
		{"pointer MarshalJSON, not addressable", compatHolder{}, "{\n  \"p\": {}\n}\n"},
		{"pointer MarshalJSON, addressable", &compatHolder{}, "{\n  \"p\": \"called\"\n}\n"},
		{"byte array", [3]byte{1, 2, 3}, "[\n  1,\n  2,\n  3\n]\n"},
	} {
		if got := formatJSONString(t, tc.data); got != tc.want {
			t.Errorf("%s:\n%s\nwant:\n%s", tc.name, got, tc.want)
		}
		var b bytes.Buffer
		if err := FormatData(&b, tc.data, FormatText); err != nil {
			t.Errorf("%s: text output failed: %v", tc.name, err)
		}
	}
}

type compatSecret struct {
	User     string  `json:"user"`
	Password string  `json:"-"`
	token    string  //nolint:unused // must never be printed
	Ratio    float64 `json:"ratio"`
	Timeout  time.Duration
	Inner    *compatSecretInner `json:"inner,omitempty"`
	Skip     string             `json:"skip,omitempty"`
}

type compatSecretInner struct {
	Key  string `json:"-"`
	Name string `json:"name"`
	Ch   chan int
}

// When JSON cannot encode a value (here a NaN and a channel), text output
// walks it instead of printing %v, so json:"-" and unexported fields stay
// hidden.
func TestFormatTextFallbackHonorsJSONTags(t *testing.T) {
	data := compatSecret{User: "ann", Password: "hunter2", token: "t0k3n", Ratio: math.NaN(), Timeout: time.Minute,
		Inner: &compatSecretInner{Key: "k3y", Name: "inner"}}
	var b bytes.Buffer
	if err := FormatData(&b, data, FormatText); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	for _, secret := range []string{"hunter2", "t0k3n", "k3y", "Password", "token", "Key", "skip"} {
		if strings.Contains(got, secret) {
			t.Errorf("text output shows %q:\n%s", secret, got)
		}
	}
	for _, want := range []string{"user = ann\n", "ratio = NaN\n", "Timeout = 1m0s\n", "inner.name = inner\n", "inner.Ch = "} {
		if !strings.Contains(got, want) {
			t.Errorf("text output lacks %q:\n%s", want, got)
		}
	}
	// JSON output reports the error and prints nothing, as before.
	b.Reset()
	if err := FormatData(&b, data, FormatJSON); err == nil || b.Len() != 0 {
		t.Errorf("JSON of a NaN: err %v, output %q", err, b.String())
	}
}
