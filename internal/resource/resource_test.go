package resource

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
)

func mustDecode(t testing.TB, s string) any {
	t.Helper()
	v, err := Decode([]byte(s))
	if err != nil {
		t.Fatalf("Decode(%s): %v", s, err)
	}
	return v
}

func TestDecode(t *testing.T) {
	for _, tt := range []struct{ in, repr string }{
		{`null`, `None`},
		{`true`, `True`},
		{`0`, `0`},
		{`-0`, `0`},
		{`-0.0`, `-0.0`},
		{`1.0`, `1.0`},
		{`1e2`, `100.0`},
		{`1E400`, `inf`},
		{`123456789012345678901234567890`, `123456789012345678901234567890`},
		{`"aé😀\ud800x"`, `'aé😀` + "�" + `x'`},
		{`{"b":1,"a":2,"b":3}`, `{'b': 3, 'a': 2}`},
		{` [1, "two", [], {}] `, `[1, 'two', [], {}]`},
	} {
		if got := Repr(mustDecode(t, tt.in)); got != tt.repr {
			t.Errorf("Repr(Decode(%s)) = %s, want %s", tt.in, got, tt.repr)
		}
	}
	for _, bad := range []string{``, `nul`, `01`, `1.`, `.5`, `[1,]`, `{"a"}`, `"a` + "\n" + `"`, `NaN`, `Infinity`, `1 2`, `"\x"`} {
		if _, err := Decode([]byte(bad)); err == nil {
			t.Errorf("Decode(%q) succeeded", bad)
		}
	}
	// json.loads accepts NaN and Infinity, which metadata values may hold.
	if v, err := decodePython([]byte(`[NaN, Infinity, -Infinity]`)); err != nil || Repr(v) != `[nan, inf, -inf]` {
		t.Errorf("decodePython = %v, %v", v, err)
	}
}

func TestRepr(t *testing.T) {
	for _, tt := range []struct {
		v    any
		want string
	}{
		{"it's", `"it's"`},
		{`both ' and "`, `'both \' and "'`},
		{"tab\tnew\nret\rnul\x00del\x7f", `'tab\tnew\nret\rnul\x00del\x7f'`},
		{"\u00a0\u00ad\u2028é😀", `'\xa0\xad\u2028é😀'`},
		{1.5e16, `1.5e+16`},
		{1e15, `1000000000000000.0`},
		{0.0001, `0.0001`},
		{0.00001, `1e-05`},
		{math.Inf(-1), `-inf`},
		{false, `False`},
	} {
		if got := Repr(tt.v); got != tt.want {
			t.Errorf("Repr(%#v) = %s, want %s", tt.v, got, tt.want)
		}
	}
	if Str("x") != "x" || Str(nil) != "None" || StringizeForSearch(nil) != "null" {
		t.Error("Str or StringizeForSearch")
	}
}

func TestNumbers(t *testing.T) {
	for _, tt := range []struct {
		in    string
		int   string // "" when int() fails
		float string // "" when float() fails
	}{
		{"42", "42", "42.0"},
		{" +7\n", "7", "7.0"},
		{"1_000", "1000", "1000.0"},
		{"1__0", "", ""},
		{"0x10", "", ""},
		{"1.5", "", "1.5"},
		{".5e1", "", "5.0"},
		{"-inf", "", "-inf"},
		{"Infinity", "", "inf"},
		{"nan", "", "nan"},
		{"1e400", "", "inf"},
		{"\x1c7", "", ""}, // str.isspace, but not stripped by int()
		{"abc", "", ""},
	} {
		n, ok, err := ParseInt(tt.in)
		if err != nil || ok != (tt.int != "") || (ok && n.String() != tt.int) {
			t.Errorf("ParseInt(%q) = %v, %v, %v; want %q", tt.in, n, ok, err, tt.int)
		}
		f, ok, err := ParseFloat(tt.in)
		if err != nil || ok != (tt.float != "") || (ok && FloatRepr(f) != tt.float) {
			t.Errorf("ParseFloat(%q) = %v, %v, %v; want %q", tt.in, f, ok, err, tt.float)
		}
	}
	var unsupported *UnsupportedError
	if _, _, err := ParseInt("١٢"); !errors.As(err, &unsupported) {
		t.Errorf("ParseInt of Arabic-Indic digits: %v, want an *UnsupportedError", err)
	}

	big := mustDecode(t, `9007199254740993`)
	n, _ := NumberOf(big)
	f, _ := NumberOf(9007199254740992.0)
	if c, ok := n.Compare(f); !ok || c != 1 {
		t.Errorf("an int and a float compare exactly: got %d, %v", c, ok)
	}
	if _, ok := (Number{Float: math.NaN()}).Compare(n); ok {
		t.Error("NaN is unordered")
	}
	if !Equal(true, mustDecode(t, `1`)) || !Equal(mustDecode(t, `1`), 1.0) || Equal("1", mustDecode(t, `1`)) {
		t.Error("Equal follows Python's ==")
	}
}

func TestCaseConversions(t *testing.T) {
	for _, tt := range []struct{ in, camel, snake string }{
		{"project_id", "projectId", "project_id"},
		{"projectId", "projectId", "project_id"},
		{"URLPath", "URLPath", "url_path"},
		{"HTTPServerError", "HTTPServerError", "http_server_error"},
		{"getHTTPResponseCode", "getHTTPResponseCode", "get_http_response_code"},
		{"foo_bAR", "fooBar", "foo_b_ar"},
		{"ΟΔΟΣ", "ΟΔΟΣ", "οδος"},
	} {
		if got := CamelCase(tt.in); got != tt.camel {
			t.Errorf("CamelCase(%q) = %q, want %q", tt.in, got, tt.camel)
		}
		if got := SnakeCase(tt.in); got != tt.snake {
			t.Errorf("SnakeCase(%q) = %q, want %q", tt.in, got, tt.snake)
		}
	}
}

func TestNormalizeForSearch(t *testing.T) {
	for _, tt := range []struct {
		v    any
		html bool
		want string
	}{
		{"Café NAÏVE", false, "cafe naive"},
		{"ﬁ①Ａ", false, "fi1a"},
		{"İstanbul", false, "istanbul"},
		{"<b>Bold</b> text", true, "bold text"},
		{"<b>Bold</b> text", false, "<b>bold</b> text"},
		{nil, false, "null"},
		{true, false, "true"},
		{[]any{"A"}, false, "['a']"},
	} {
		if got := NormalizeForSearch(tt.v, tt.html); got != tt.want {
			t.Errorf("NormalizeForSearch(%#v, %v) = %q, want %q", tt.v, tt.html, got, tt.want)
		}
	}
}

func TestKey(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"a.b[0].c[].d", "a.b[0].c[].d"},
		{`"k.e.y".value`, "k.e.y.value"},
		{"labels.my-label", "labels.my-label"},
		{"a[-1]", "a[-1]"},
		{".", ""},
		{" a ", "a"},
	} {
		key, err := ParseKey(tt.in)
		if err != nil || key.String() != tt.want {
			t.Errorf("ParseKey(%q) = %q, %v; want %q", tt.in, key.String(), err, tt.want)
		}
	}
	var syntax *SyntaxError
	var unsupported *UnsupportedError
	for _, tt := range []struct {
		in          string
		unsupported bool
	}{
		{"a.", false}, {"a..b", false}, {"a]", false}, {"a[0", false}, {"a:b", false}, {"a b", false}, {`"a`, false},
		{"*a", true}, {"a[x]", true}, {"a[1.5]", true}, {`a["0"]`, true},
	} {
		_, err := ParseKey(tt.in)
		switch {
		case tt.unsupported && !errors.As(err, &unsupported):
			t.Errorf("ParseKey(%q): %v, want an *UnsupportedError", tt.in, err)
		case !tt.unsupported && !errors.As(err, &syntax):
			t.Errorf("ParseKey(%q): %v, want a *SyntaxError", tt.in, err)
		}
	}
	if _, err := ParseKey("a.b c"); err == nil || !strings.Contains(err.Error(), "[a.b *HERE* c]") {
		t.Errorf("ParseKey error marks the position: %v", err)
	}
}

func TestGet(t *testing.T) {
	doc := mustDecode(t, `{
		"name": "vm-1", "projectId": "p", "n": 10, "labels": {"env": "prod"},
		"disks": [{"name": "boot", "size": 10}, {"name": "data"}, {"type": "x"}],
		"other": [{"type": "x"}, {"name": "a"}, {"name": ""}, {"name": "b"}],
		"metadata": {"items": [{"key": "startup", "value": "echo"}, {"key": "cfg", "value": "{\"a\": 1}"}]},
		"bindings": [{"role": "editor", "members": ["u1"]}, {"role": "owner", "members": ["u2"]}],
		"s": "héllo"
	}`)
	for _, tt := range []struct{ key, want string }{
		{"name", `'vm-1'`},
		{"project_id", `'p'`},
		{"labels.env", `'prod'`},
		{"labels.nope", `None`},
		{"n.real", `10`},
		{"n.denominator", `1`},
		{"disks.name", `['boot', 'data', None]`},
		{"disks[1].name", `'data'`},
		{"disks[-1].type", `'x'`},
		{"disks[3]", `None`},
		{"disks[].size", `[10, None, None]`},
		{"other.name", `['a', 'b']`},
		{"metadata.startup", `'echo'`},
		{"metadata.cfg.a", `1`},
		{"bindings.role.owner.members", `['u2']`},
		{"s[1]", `'é'`},
		{"labels[]", `{'env': 'prod'}`},
		{"labels[].x", `[None]`},
		{".", ""},
	} {
		key, err := ParseKey(tt.key)
		if err != nil {
			t.Fatalf("ParseKey(%q): %v", tt.key, err)
		}
		got, err := Get(doc, key)
		if err != nil {
			t.Fatalf("Get(%q): %v", tt.key, err)
		}
		if tt.key == "." {
			if got != doc {
				t.Errorf("Get(.) is not the resource")
			}
			continue
		}
		if Repr(got) != tt.want {
			t.Errorf("Get(%q) = %s, want %s", tt.key, Repr(got), tt.want)
		}
	}
	var unsupported *UnsupportedError
	key, _ := ParseKey("n.__doc__")
	if _, err := Get(doc, key); !errors.As(err, &unsupported) {
		t.Errorf("Get(n.__doc__): %v, want an *UnsupportedError", err)
	}
}

func TestSortBy(t *testing.T) {
	list := mustDecode(t, `[{"n":10,"s":"b"},{"n":9,"s":"a"},{"n":null,"s":"b"},{"s":"a"},{"n":"Z","s":"a"},{"n":1.5,"s":"c"}]`).([]any)
	for _, tt := range []struct {
		by   string
		want string
	}{
		{"", "[0 1 2 3 4 5]"},
		{"n", "[5 0 1 2 3 4]"}, // by text: "1.5" < "10" < "9" < "None" = "None" < "Z"
		{"~n", "[4 2 3 1 0 5]"},
		{"s,n", "[1 3 4 0 2 5]"},
		{"~s,n", "[5 0 2 1 3 4]"},
		{"s,~n", "[4 3 1 2 0 5]"},
	} {
		keys, err := ParseSortBy(tt.by)
		if err != nil {
			t.Fatal(err)
		}
		order, err := SortBy(list, keys)
		if err != nil || fmt.Sprint(order) != tt.want {
			t.Errorf("SortBy(%q) = %v, %v; want %s", tt.by, order, err, tt.want)
		}
	}
	for _, bad := range []string{"a,,b", ",", "~", " ~a", "a:b", "a.", "*a"} {
		if _, err := ParseSortBy(bad); err == nil {
			t.Errorf("ParseSortBy(%q) succeeded", bad)
		}
	}
}

func FuzzKey(f *testing.F) {
	for _, s := range []string{"a.b[0].c[].d", `"k.e.y".v`, "a[-1]", ".", "a b", `a\"b`, "*a", "a[x]", "", "[", "a.b(1)"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		l := NewLexer(s)
		key, err := l.Key()
		if err != nil {
			var syntax *SyntaxError
			var unsupported *UnsupportedError
			if !errors.As(err, &syntax) && !errors.As(err, &unsupported) {
				t.Fatalf("Key(%q): unexpected error type %T: %v", s, err, err)
			}
			return
		}
		if l.pos < 0 || l.pos > len(l.expr) {
			t.Fatalf("Key(%q) left the position at %d", s, l.pos)
		}
		for _, p := range key {
			if p.IsName() && p.Name == "" {
				t.Fatalf("Key(%q) has an empty name", s)
			}
		}
		doc := mustDecode(t, `{"a": {"b": [{"c": [1, {"d": "x"}]}]}, "k.e.y": {"v": 2}}`)
		if _, err := Get(doc, key); err != nil {
			var unsupported *UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("Get(%q): %v", s, err)
			}
		}
	})
}

func FuzzDecode(f *testing.F) {
	for _, s := range []string{`{"a":[1,2.5,"x",null,true]}`, `"\ud800"`, `1e999`, `[`, `{"a":1,"a":2}`, `-0`} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		v, err := Decode(data)
		if json.Valid(data) != (err == nil) {
			t.Fatalf("Decode(%q): err = %v, but json.Valid = %v", data, err, json.Valid(data))
		}
		if err == nil {
			_ = Repr(v)
			_ = NormalizeForSearch(v, true)
		}
	})
}
