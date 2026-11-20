package resource

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2/internal/gcloudtest"
)

// These tests compare the port with the gcloud library itself, or without
// gcloud, with its results recorded in testdata; see internal/gcloudtest.

// getScript applies resource_property.Get to each case and prints the
// repr of the result, or the error.
const getScript = `
from googlecloudsdk.core.resource import resource_lex, resource_property
out = []
for c in json.load(sys.stdin):
    try:
        key = resource_lex.Lexer(c["key"]).Key()
        out.append({"repr": repr(resource_property.Get(json.loads(c["json"]), key, None))})
    except Exception as e:
        out.append({"error": type(e).__name__ + ": " + str(e)})
print(json.dumps(out))
`

type getCase struct {
	JSON string `json:"json"`
	Key  string `json:"key"`
}

type gcloudResult struct {
	Repr  string `json:"repr"`
	Error string `json:"error"`
}

var getDiffCases = func() []getCase {
	docs := []string{
		`{"name":"a","projectId":"p","project_id":"q","labels":{"env":"prod","team":"x"},"n":10,"f":1.5,"b":true,"z":null}`,
		`{"items":[{"key":"startup-script","value":"echo"},{"key":"cfg","value":"{\"a\":[1,2]}"},{"key":"bad","value":"{"}]}`,
		`{"metadata":{"items":[{"key":"k1","value":"v1"},{"key":"json","value":"{\"x\":{\"y\":3}}"}]}}`,
		`{"disks":[{"name":"boot","size":10,"opts":{"ssd":true}},{"name":"data","size":0},{"type":"x"}]}`,
		`{"disks":[{"type":"x"},{"name":"boot"},{"name":""},{"name":"data"}]}`,
		`{"tags":{"items":["a","b","c"]},"list":[1,[2,3],"s",null,{"k":"v"}],"empty":[],"eo":{}}`,
		`{"bindings":[{"role":"roles/editor","members":["u1","u2"]},{"role":"roles/owner","members":["u3"]}]}`,
		`{"s":"héllo","num":-0.0,"big":123456789012345678901234567890,"e":1e16,"small":1e-5,"inf":1e400}`,
		`{"camelCaseKey":1,"snake_case_key":2,"URLPath":3,"url_path":4,"aB":5}`,
		`[{"a":1},{"a":2}]`,
		`"just a string"`,
		`42`,
	}
	keys := []string{
		"name", "projectId", "project_id", "labels", "labels.env", "labels.ENV", "n", "n.real", "n.imag",
		"n.numerator", "n.denominator", "f.real", "f.imag", "b.real", "b.numerator", "z", "z.x", "missing",
		"startup-script", "cfg", "cfg.a", "cfg.a[1]", "bad", "bad.x", "metadata.k1", "metadata.json.x.y",
		"metadata.json", "metadata.nope", "disks", "disks.name", "disks.size", "disks.type", "disks[0]",
		"disks[-1]", "disks[3]", "disks[].name", "disks[0].opts.ssd", "disks.name.boot", "disks.name.boot.size",
		"disks.opts", "tags.items", "tags.items[0]", "tags.items[-4]", "list[1][0]", "list[2][0]", "list[4].k",
		"list[]", "list[].k", "empty", "empty[]", "empty[].x", "eo[]", "eo[].x", "labels[]", "labels[].x",
		"bindings.role", "bindings.role.roles/editor.members", "bindings.members", "bindings[1].members[0]",
		"s[1]", "s[-1]", "s[9]", "s[]", "num", "big", "e", "small", "inf", "camel_case_key", "snakeCaseKey",
		"url_path", "URLPath", "a_b", "aB", "[0]", "[0].a", "[].a", "a", "[-1]", "real", "imag", ".",
		`"labels".env`, `labels."env"`, "labels.env[0]", "name[0]", "n[0]", "projectId.x",
	}
	var cases []getCase
	for _, d := range docs {
		for _, k := range keys {
			cases = append(cases, getCase{d, k})
		}
	}
	return cases
}()

func TestGetMatchesGcloud(t *testing.T) {
	var want []gcloudResult
	gcloudtest.Golden(t, "testdata/get.json", getScript, getDiffCases, &want)
	compared := 0
	for i, c := range getDiffCases {
		w := want[i]
		v, err := Decode([]byte(c.JSON))
		if err != nil {
			t.Fatalf("Decode(%s): %v", c.JSON, err)
		}
		key, err := NewLexer(c.Key).Key()
		var unsupported *UnsupportedError
		switch {
		case errors.As(err, &unsupported):
			continue
		case err != nil && w.Error == "":
			t.Errorf("ParseKey(%q): %v; gcloud got %s", c.Key, err, w.Repr)
			continue
		case err != nil:
			continue
		}
		got, err := Get(v, key)
		switch {
		case errors.As(err, &unsupported):
			continue
		case err != nil:
			t.Errorf("Get(%s, %q): %v; gcloud: %+v", c.JSON, c.Key, err, w)
			continue
		case w.Error != "":
			t.Errorf("Get(%s, %q) = %s; gcloud failed: %s", c.JSON, c.Key, Repr(got), w.Error)
			continue
		}
		compared++
		if Repr(got) != w.Repr {
			t.Errorf("Get(%s, %q) = %s, gcloud %s", c.JSON, c.Key, Repr(got), w.Repr)
		}
	}
	t.Logf("compared %d of %d cases with gcloud", compared, len(getDiffCases))
}

// pythonScript evaluates Python helpers on strings: repr, the case
// conversions, int(), float() and NormalizeForSearch.
const pythonScript = `
from googlecloudsdk.core.resource import resource_property, resource_expr
def num(f, s):
    try:
        return repr(f(s))
    except ValueError:
        return "ValueError"
out = []
for s in json.load(sys.stdin):
    out.append({
        "repr": repr(s),
        "camel": resource_property.ConvertToCamelCase(s),
        "snake": resource_property.ConvertToSnakeCase(s),
        "int": num(int, s),
        "float": num(float, s),
        "norm": resource_expr.NormalizeForSearch(s, html=True),
    })
print(json.dumps(out))
`

func TestPythonHelpersMatchGcloud(t *testing.T) {
	inputs := []string{
		"", "a", "it's", `say "hi"`, `both ' and "`, "back\\slash", "tab\there", "new\nline", "\r", "\x00\x1f\x7f",
		"\u0080\u009f\u00a0\u00ad", "é", "Café", "naïve", "ﬁ", "①", "Ａ", "ǅ", "İstanbul", "ΟΔΟΣ", "\u2028", "\ufeff",
		"😀", "\U000e0001", "projectId", "project_id", "URLPath", "HTTPServerError", "getHTTPResponseCode", "a1B",
		"aBC", "ABC", "aBc", "a_b_c", "__doc__", "_private", "already_snake", "camelCase2Snake", "x\n", "AB\n",
		"foo_bAR", "a1b_c2d", "ß_x", "0", "007", "-0", "+5", " 42 ", "1_000", "1__0", "_1", "1_", "1.5", "1.",
		".5", "-.5e3", "1e5", "1E-5", "1e", "e5", "inf", "-Infinity", "NaN", "+nan", "1_0.5", "1._5", "0x10",
		"1e400", "-1e400", "1e-400", "١٢", "\u00a012\u00a0", "\x1c7\x1f", "<b>bold</b> text", "a<b", "<<x>>y",
		"123456789012345678901234567890", "9007199254740993", "4.35", "0.1", "1e16", "1e15", "123456789.123456789",
	}
	var want []map[string]string
	gcloudtest.Golden(t, "testdata/python.json", pythonScript, inputs, &want)
	for i, s := range inputs {
		w := want[i]
		if got := Repr(s); got != w["repr"] {
			t.Errorf("Repr(%q) = %s, Python %s", s, got, w["repr"])
		}
		if got := CamelCase(s); got != w["camel"] {
			t.Errorf("CamelCase(%q) = %q, gcloud %q", s, got, w["camel"])
		}
		if got := SnakeCase(s); got != w["snake"] {
			t.Errorf("SnakeCase(%q) = %q, gcloud %q", s, got, w["snake"])
		}
		if got := NormalizeForSearch(s, true); got != w["norm"] {
			t.Errorf("NormalizeForSearch(%q) = %q, gcloud %q", s, got, w["norm"])
		}
		checkNum := func(name string, got string, ok bool, err error) {
			t.Helper()
			var unsupported *UnsupportedError
			switch {
			case errors.As(err, &unsupported):
			case err != nil:
				t.Errorf("%s(%q): %v", name, s, err)
			case !ok && w[name] != "ValueError":
				t.Errorf("%s(%q) failed, Python %s", name, s, w[name])
			case ok && got != w[name]:
				t.Errorf("%s(%q) = %s, Python %s", name, s, got, w[name])
			}
		}
		n, ok, err := ParseInt(s)
		checkNum("int", fmt.Sprint(n), ok, err)
		f, ok, err := ParseFloat(s)
		checkNum("float", FloatRepr(f), ok, err)
	}
}

// floatScript prints the repr of floats.
const floatScript = `
out = [repr(float(s)) for s in json.load(sys.stdin)]
print(json.dumps(out))
`

func TestFloatReprMatchesPython(t *testing.T) {
	var inputs []string
	for _, f := range []float64{0, 1, -1, 0.5, 100, 1e15, 1e16, 1.5e16, 1e-4, 1e-5, 1.25e-5, 123456789012345.6, 1234567890123456.7,
		math.MaxFloat64, math.SmallestNonzeroFloat64, 2.5e-308, 1.0 / 3, 2.0 / 3, 0.1 + 0.2, 9007199254740993, -0.0001, 12345.678e10} {
		inputs = append(inputs, fmt.Sprint(f))
	}
	var want []string
	gcloudtest.Golden(t, "testdata/float.json", floatScript, inputs, &want)
	for i, s := range inputs {
		f, ok, err := ParseFloat(s)
		if !ok || err != nil {
			t.Fatalf("ParseFloat(%q) failed", s)
		}
		if got := FloatRepr(f); got != want[i] {
			t.Errorf("FloatRepr(%s) = %s, Python %s", s, got, want[i])
		}
	}
}

// sortScript sorts each case's resources with gcloud's display code.
const sortScript = `
import six
from googlecloudsdk.core.resource import resource_lex, resource_property
def get_key(r, key):
    value = resource_property.Get(r, key, None)
    try:
        assert None < value
        return value
    except (AssertionError, TypeError):
        return six.text_type(value)
out = []
for c in json.load(sys.stdin):
    data = json.loads(c["json"])
    keys = []
    for name in c["sort_by"].split(","):
        reverse = name.startswith("~")
        name = name.lstrip("~").replace("[]", "[0]")
        keys.append((resource_lex.Lexer(name).Key(), reverse))
    groups, group_keys, group_reverse = [], [], False
    for key, reverse in keys:
        if not group_keys:
            group_reverse = reverse
        elif group_reverse != reverse:
            groups.insert(0, (group_keys, group_reverse))
            group_keys, group_reverse = [], reverse
        group_keys.append(key)
    if group_keys:
        groups.insert(0, (group_keys, group_reverse))
    order = list(range(len(data)))
    for ks, rev in groups:
        order = sorted(order, key=lambda i: [get_key(data[i], k) for k in ks], reverse=rev)
    out.append(order)
print(json.dumps(out))
`

type sortCase struct {
	JSON   string `json:"json"`
	SortBy string `json:"sort_by"`
}

func TestSortByMatchesGcloud(t *testing.T) {
	docs := []string{
		`[{"n":10,"s":"b"},{"n":9,"s":"a"},{"n":1.5,"s":"b"},{"n":null,"s":"a"},{"s":"c"},{"n":"a","s":"B"},{"n":true,"s":"b"},{"n":100.0,"s":"a"},{"n":-1,"s":"é"},{"n":[2,1],"s":"a"},{"n":{"x":1},"s":"b"}]`,
		`[{"name":"zeta","zone":"us-b","tags":["x","y"]},{"name":"alpha","zone":"us-a","tags":["z"]},{"name":"Beta","zone":"us-a"},{"name":"alpha","zone":"eu-a","tags":[]}]`,
		`[{"a":{"b":2}},{"a":{"b":1}},{"a":{"c":3}},{"a":[{"b":0}]},{"a":"str"}]`,
	}
	sorts := []string{"n", "~n", "s", "s,n", "~s,n", "s,~n", "~s,~n", "n,s", "name", "~name", "zone,name", "zone,~name",
		"~zone,name,~tags", "tags", "tags[]", "tags[0]", "tags[1]", "a.b", "~a.b", "a.c,a.b", "missing", "~missing,name"}
	var cases []sortCase
	for _, d := range docs {
		for _, s := range sorts {
			cases = append(cases, sortCase{d, s})
		}
	}
	var want [][]int
	gcloudtest.Golden(t, "testdata/sort.json", sortScript, cases, &want)
	for i, c := range cases {
		v, err := Decode([]byte(c.JSON))
		if err != nil {
			t.Fatal(err)
		}
		keys, err := ParseSortBy(c.SortBy)
		if err != nil {
			t.Errorf("ParseSortBy(%q): %v", c.SortBy, err)
			continue
		}
		got, err := SortBy(v.([]any), keys)
		if err != nil {
			t.Errorf("SortBy(%s, %q): %v", c.JSON, c.SortBy, err)
			continue
		}
		if fmt.Sprint(got) != fmt.Sprint(want[i]) {
			t.Errorf("SortBy(%s, %q) = %v, gcloud %v", c.JSON, c.SortBy, got, want[i])
		}
	}
}

// keyScript parses keys with gcloud's lexer.
const keyScript = `
from googlecloudsdk.core.resource import resource_lex
out = []
for s in json.load(sys.stdin):
    try:
        lex = resource_lex.Lexer(s)
        key = lex.Key()
        out.append({"key": repr(key), "rest": s[lex.GetPosition():]})
    except Exception as e:
        out.append({"error": type(e).__name__ + ": " + str(e)})
print(json.dumps(out))
`

func TestKeyMatchesGcloud(t *testing.T) {
	inputs := []string{
		"a", "a.b", "a.b.c", "a[0]", "a[-1]", "a[]", "a[ ]", "a[0][1].b", "a.b[]", `"k.e.y".value`, `'a b'.c`,
		"a b", "a:b", "a=b", "a.", ".", ".a", "a..b", "[0]", "[]", "a]", "a[0", "a[0]x", "my-label", "labels.my-label",
		`a\.b`, `"a\"b"`, `'it\'s'`, "a.b(1)", "a(1)", "x y", " a", "a ", "a.  b", "a[007]", "a[+1]", "a[1_0]",
		"\ta", "é.ü", "a.b.c.d.e.f", "1.2", "a.1", "a[0].1", `a."0"`, "a.[0]", "a[0].[1]",
	}
	var want []map[string]string
	gcloudtest.Golden(t, "testdata/key.json", keyScript, inputs, &want)
	for i, s := range inputs {
		w := want[i]
		l := NewLexer(s)
		key, err := l.Key()
		var unsupported *UnsupportedError
		switch {
		case errors.As(err, &unsupported):
			continue
		case err != nil && w["error"] == "":
			t.Errorf("Key(%q): %v; gcloud %s", s, err, w["key"])
			continue
		case err != nil:
			continue
		case w["error"] != "":
			t.Errorf("Key(%q) = %v; gcloud failed: %s", s, key, w["error"])
			continue
		}
		if got := pyKey(key); got != w["key"] {
			t.Errorf("Key(%q) = %s, gcloud %s", s, got, w["key"])
		}
		if rest := string(l.expr[l.pos:]); rest != w["rest"] {
			t.Errorf("Key(%q) left %q, gcloud %q", s, rest, w["rest"])
		}
	}
}

// pyKey writes key as gcloud's Python list.
func pyKey(key Key) string {
	parts := make([]string, len(key))
	for i, p := range key {
		switch {
		case p.Slice:
			parts[i] = "None"
		case p.Index != nil:
			parts[i] = p.Index.String()
		default:
			parts[i] = Repr(p.Name)
		}
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
