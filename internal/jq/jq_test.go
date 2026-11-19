package jq

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"sync"
	"testing"
)

// run is Parse followed by Write.
func run(t *testing.T, filter, input string) (string, error) {
	t.Helper()
	q, err := Parse(filter)
	if err != nil {
		t.Fatalf("Parse(%q): %v", filter, err)
	}
	var b bytes.Buffer
	err = q.Write(&b, []byte(input))
	return b.String(), err
}

// These expectations were checked against jq 1.7.1; they run without jq.
var goldenCases = []struct {
	filter, input, want string
	wantErr             string // a substring of the runtime error, if any
}{
	{".", `{"b":1,"a":[1,{"c":null}],"e":{},"f":[]}`, "{\n  \"b\": 1,\n  \"a\": [\n    1,\n    {\n      \"c\": null\n    }\n  ],\n  \"e\": {},\n  \"f\": []\n}\n", ""},
	{".a", `{"a":"x"}`, "x\n", ""},
	{".a", `{"a":"x\ty"}`, "x\ty\n", ""},
	{"[.a]", `{"a":"x\ty\u007f\u001f\"\\é"}`, "[\n  \"x\\ty\\u007f\\u001f\\\"\\\\é\"\n]\n", ""},
	{".a.b", `{"a":null}`, "null\n", ""},
	{".a.b", `{"a":"s"}`, "", `Cannot index string with string "b"`},
	{".[1]", `[1,2,3]`, "2\n", ""},
	{".[-1]", `[1,2,3]`, "3\n", ""},
	{".[1.7], .[-1.5], .[9]", `[1,2,3]`, "2\n3\nnull\n", ""},
	{".[0]", `{"a":1}`, "", "Cannot index object with number"},
	{".[]", `{"b":1,"a":2}`, "1\n2\n", ""},
	{".[]", `3`, "", "Cannot iterate over number (3)"},
	{"[.[].a?]", `[1,{"a":2},3]`, "[\n  2\n]\n", ""},
	{"[(.[].a)?]", `[1,{"a":2},3]`, "[]\n", ""},
	{"[..]", `[1,[2]]`, "[\n  [\n    1,\n    [\n      2\n    ]\n  ],\n  1,\n  [\n    2\n  ],\n  2\n]\n", ""},
	{".a, .b", `{"a":1,"b":2}`, "1\n2\n", ""},
	{"[(1,2) < (3,0)]", `null`, "[\n  true,\n  true,\n  false,\n  false\n]\n", ""},
	{"[(false,true) or (false,true)]", `null`, "[\n  false,\n  true,\n  true\n]\n", ""},
	{`"\(1,2)\(3,4)"`, `null`, "13\n23\n14\n24\n", ""},
	{"{a:(1,2),b:(3,4)} | [.a,.b] | @csv", `null`, "1,3\n1,4\n2,3\n2,4\n", ""},
	{`{"\(1,2)"}`, `{"1":"x"}`, "{\n  \"1\": \"x\"\n}\n{\n  \"2\": null\n}\n", ""},
	{"{(1): empty}", `null`, "", ""}, // a parse error; see TestParseErrors
	{"1.0, 1e2, 1e-5, 1.5e-7, 100000000000000000001", `null`, "1.0\n1E+2\n0.00001\n1.5E-7\n100000000000000000001\n", ""},
	// jq 1.8: unary minus and length keep a literal's digits; -0 is 0.
	{"-1.0, -0, (1e1000 | length), (123456789012345678 | length)", `null`, "-1.0\n0\n1E+1000\n123456789012345678\n", ""},
	{"-0.0, - -1, (-1.50 | length), (-0 | tostring)", `null`, "0.0\n1\n1.50\n0\n", ""},
	{"length", `-123456789012345678901234567890`, "123456789012345678901234567890\n", ""},
	{"-1 == -1.0, -0 == 0, (-2 | length) < 3", `null`, "true\ntrue\ntrue\n", ""},
	{". == 100000000000000000000, . == 100000000000000000001, 1.0 == 1", `100000000000000000001`, "false\ntrue\ntrue\n", ""},
	{"., length", `1e-7`, "1E-7\n1E-7\n", ""},
	{"sort", `[3,1,null,"a",true,false,[1],{"a":1},1.0,1]`, "[\n  null,\n  false,\n  true,\n  1,\n  1.0,\n  1,\n  3,\n  \"a\",\n  [\n    1\n  ],\n  {\n    \"a\": 1\n  }\n]\n", ""},
	{"sort_by(.a) | map(.b)", `[{"a":2,"b":1},{"a":1,"b":2},{"a":1,"b":3}]`, "[\n  2,\n  3,\n  1\n]\n", ""},
	{"keys, keys_unsorted", `{"b":1,"a":2}`, "[\n  \"a\",\n  \"b\"\n]\n[\n  \"b\",\n  \"a\"\n]\n", ""},
	{"to_entries | map(\"\\(.key)=\\(.value)\") | join(\",\")", `{"b":1,"a":"x"}`, "b=1,a=x\n", ""},
	{"join(\"-\")", `[1,null,"a",true]`, "1--a-true\n", ""},
	{"join(\",\")", `[[1]]`, "", `cannot be added`},
	{"map(has(\"a\"))", `[{"a":null},{}]`, "[\n  true,\n  false\n]\n", ""},
	{"has(0), has(5)", `[1]`, "true\nfalse\n", ""},
	{"length", `"héllo"`, "5\n", ""},
	{"length", `true`, "", "boolean (true) has no length"},
	{"ascii_downcase, ascii_upcase", `"aBc É"`, "abc É\nABC É\n", ""},
	{"[.[] | select(. > 1)]", `[1,2,3]`, "[\n  2,\n  3\n]\n", ""},
	{"first, last, first(.[]|select(.>1)), [last(empty)]", `[1,2,3]`, "1\n3\n2\n[]\n", ""}, // jq 1.8: last(empty) is empty
	// jq 1.8's grammar: an object value is an expression without a comma,
	// and if ... end and e? are terms that take suffixes.
	{"{a: .x == 1, b: .x // 1, c: if . then 1 else 2 end, d: 1?, e: -1.0 | length}", `null`,
		"{\n  \"a\": false,\n  \"b\": 1,\n  \"c\": 2,\n  \"d\": 1,\n  \"e\": 1.0\n}\n", ""},
	{"{a: .x or .y and true}", `{"x":false,"y":true}`, "{\n  \"a\": true\n}\n", ""},
	{"if true then {\"a\":1} else 2 end.a, ([.[0]])?[0]", `[5]`, "1\n5\n", ""},
	{"(1)?.b", `null`, "", `Cannot index number with`},
	{"1 # the comment goes on \\\n 2", `null`, "1\n", ""},
	{"[.[] | .a?.b]", `[{"a":{"b":1}},3]`, "[\n  1\n]\n", ""},
	// jq 1.8's lexer: CR is whitespace, and a backslash continues a comment.
	{"[1,\r\n2]", `null`, "[\n  1,\n  2\n]\n", ""},
	{"[1, # comment \\\n 2,\n 3]", `null`, "[\n  1,\n  3\n]\n", ""},
	{"[1, # comment \\\r\n 2,\r\n 3]", `null`, "[\n  1,\n  3\n]\n", ""},
	{"[1, # comment \\\\\n 2]", `null`, "[\n  1,\n  2\n]\n", ""},
	{"[1, # comment\r\n 2]", `null`, "[\n  1,\n  2\n]\n", ""},
	{".a // \"none\"", `{"a":false}`, "none\n", ""},
	{"if . then \"t\" elif . == false then \"f\" else \"n\" end", `null`, "n\n", ""},
	{"[.[] | type]", `[null,true,1,"s",[],{}]`, "[\n  \"null\",\n  \"boolean\",\n  \"number\",\n  \"string\",\n  \"array\",\n  \"object\"\n]\n", ""},
	{"[.[] | values]", `[null,false,0]`, "[\n  false,\n  0\n]\n", ""},
	{"tostring, tojson", `{"a":[1.0,"x"]}`, "{\"a\":[1.0,\"x\"]}\n{\"a\":[1.0,\"x\"]}\n", ""},
	{"@tsv, @csv", `["a\tb","c\"d",null,true,1.5,"\u0000"]`, "a\\tb\tc\"d\t\ttrue\t1.5\t\\0\n\"a\tb\",\"c\"\"d\",,true,1.5,\"\\0\"\n", ""},
	{"@tsv", `"x"`, "", "cannot be tsv-formatted, only array"},
	{"@json, @text", `"x"`, "\"x\"\nx\n", ""},
	{`test("a$"), test("^b"), test("A")`, `"a\n"`, "true\nfalse\nfalse\n", ""},
	{`test("^\\p{L}+$")`, `"Ελληνικά"`, "true\n", ""},
	{`test(.)`, `"a"`, "true\n", ""},
	{`test(.)`, `"\\d"`, "", `regex \d is not supported in clix --jq`},
	{`test(.)?`, `"\\d"`, "", `regex \d is not supported in clix --jq`},
	{`[.[] | test(.)?]`, `["a", "{"]`, "", `regex { that is not a quantifier`},
	{`.[{}]?`, `[1]`, "", "slices (.[a:b]) are not supported in clix --jq"},
	{`test(["a", "x"])?`, `"a"`, "", "regex flags"},
	{`test(.)?`, `"("`, "", ""},
	{`test("a")`, `1`, "", "cannot be matched, as it is not a string"},
	{".[[1]]", `[1,2,1]`, "[\n  0,\n  2\n]\n", ""},
	{".[{}]", `null`, "null\n", ""},
	{".[{}]", `[1]`, "", "slices (.[a:b]) are not supported in clix --jq"},
	{".a?.b, 1", `"x"`, "1\n", ""},
	{"(.a, 2)?, 3", `"x"`, "3\n", ""},
	{"[(1,2)? | keys]", `null`, "", "number (1) has no keys"},
	{".[0], (.[1] | keys)", `["s", 1]`, "s\n", "number (1) has no keys"},
	{"{if, a: -1 | 2, (\"k\"): 3,}", `{"if":0}`, "{\n  \"if\": 0,\n  \"a\": 2,\n  \"k\": 3\n}\n", ""},
	{"# comment\n.a # trailing", `{"a":1}`, "1\n", ""},
}

func TestGolden(t *testing.T) {
	for _, tc := range goldenCases {
		t.Run(tc.filter, func(t *testing.T) {
			q, err := Parse(tc.filter)
			if err != nil {
				if tc.want == "" && tc.wantErr == "" {
					return // documented as a parse error
				}
				t.Fatalf("Parse(%q): %v", tc.filter, err)
			}
			var b bytes.Buffer
			err = q.Write(&b, []byte(tc.input))
			if got := b.String(); got != tc.want {
				t.Errorf("output = %q, want %q", got, tc.want)
			}
			switch {
			case tc.wantErr == "" && err != nil:
				t.Errorf("error = %v, want none", err)
			case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
				t.Errorf("error = %v, want one containing %q", err, tc.wantErr)
			}
			if err != nil {
				var re *RuntimeError
				var u *UnsupportedError
				if strings.Contains(tc.wantErr, "not supported") || strings.HasPrefix(tc.wantErr, "regex") {
					if !errors.As(err, &u) {
						t.Errorf("error %v is a %T, want an *UnsupportedError", err, err)
					}
				} else if !errors.As(err, &re) {
					t.Errorf("error %v is a %T, want a *RuntimeError", err, err)
				}
			}
		})
	}
}

func TestUnsupported(t *testing.T) {
	for _, tc := range []struct{ filter, construct string }{
		{".a + 1", "the + operator"},
		{".a - 1", "the - operator"},
		{"1 * 2", "the * operator"},
		{"1 / 2", "the / operator"},
		{"1 % 2", "the % operator"},
		{"-.a", "negation (unary -)"},
		{"-(1,2)", "negation (unary -)"},
		{".a = 1", "the = operator"},
		{".a |= 1", "the |= operator"},
		{".a += 1", "the += operator"},
		{".a //= 1", "the //= operator"},
		{".a?//1", "the ?// operator"},
		{". as $x | $x", "variable bindings (... as $x)"},
		{"$x", "variables ($x)"},
		{"$__loc__", "$__loc__"},
		{"{$x}", "variables ($x)"},
		{"reduce .[] as $x (0; 1)", "reduce"},
		{"foreach .[] as $x (0; 1)", "foreach"},
		{"def f: 1; f", "function definitions (def)"},
		{"try .a", "try/catch"},
		{"label $f | 1", "label/break"},
		{`import "a" as a; 1`, "modules (import)"},
		{`include "a"; 1`, "modules (include)"},
		{".[1:2]", "slices (.[a:b])"},
		{".[:2]", "slices (.[a:b])"},
		{`@base64`, "the format @base64"},
		{`@csv "\(.)"`, `format strings (@csv "...")`},
		{"splits(\"a\")", "the function splits/1"},
		{"limit(1; .[])", "the function limit/2"},
		{"test(\"a\"; \"i\")", "the function test/2"},
		{"ascii", "the function ascii/0"},
		{"length(1)", "the function length/1"},
		{"mod::f", "the function mod::f/0"},
		{`test("\\d")`, `regex \d`},
		{`test("\\w+")`, `regex \w`},
		{`test("\\s")`, `regex \s`},
		{`test("\\bx")`, `regex \b`},
		{`test("[[:alpha:]]")`, "regex [ inside a class"},
		{`test("(?i)a")`, "regex inline flags"},
		{`test("(?=a)")`, "regex lookahead"},
		{`test("(?<=a)b")`, "regex lookbehind"},
		{`test("(?>a)")`, "regex atomic groups"},
		{`test("(a)\\1")`, `regex \1`},
		{`test("a*+")`, "regex possessive quantifiers"},
		{`test("a**")`, "regex nested quantifiers"},
		{"not(true)", "the function not/1"},
		{`test("a{,2}")`, "regex { that is not a quantifier"},
		{`test("a{1001}")`, `regex "a{1001}"`},
		{`test("{")`, "regex { that is not a quantifier"},
		{`test("a$b")`, "regex $ before the end of the pattern"},
		{`test("(a$)*")`, "regex $ before the end of the pattern"},
		{`test("(^a)*")`, "regex anchors (^, \\A, \\z) inside a quantified group"},
		{`test("(?<n>a|\\z){2}")`, "regex anchors (^, \\A, \\z) inside a quantified group"},
		{`test("[a&&b]")`, "regex && inside a class"},
		{`test("[]a]")`, "regex ] at the start of a class"},
		{`test("[a-z-9]")`, "regex - after a range"},
		{`test("\\Z")`, `regex \Z`},
		{`test("\\h")`, `regex \h`},
		{`test("\\pL")`, `regex \p without braces`},
		{`test("\\p{C}")`, `regex \p{C}`},
		{`test("\\p{greek}")`, `regex \p{greek}`},
		{`test("a]")`, "regex unescaped ]"},
		{`test("\\x4")`, `regex \x with fewer than two hex digits`},
		{`test("\\v")`, `regex \v`},
		{`test("\\<")`, `regex \<`},
	} {
		t.Run(tc.filter, func(t *testing.T) {
			_, err := Parse(tc.filter)
			var u *UnsupportedError
			if !errors.As(err, &u) {
				t.Fatalf("Parse(%q) = %v, want an *UnsupportedError", tc.filter, err)
			}
			if !strings.HasPrefix(u.Construct, tc.construct) {
				t.Errorf("construct = %q, want it to start with %q", u.Construct, tc.construct)
			}
			if !strings.Contains(err.Error(), " not supported in clix --jq") {
				t.Errorf("message %q does not say it is not supported", err)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	for _, filter := range []string{
		"", " ", "# only a comment",
		".a .", "..a", `.."a"`, "1, # the comment goes on \\\n 2", "{a: 1, 2}", "{a: 1 2}",
		". == . == .", "1 < 2 < 3",
		"[1,]", "{1: 2}",
		"(", ")", "[", "{", "{a", "{a:", `"abc`, `"\x"`, `"\ud83d"`, `"\ude00"`, `"\(1"`, `"\()"`,
		"if . then 1", "if . then 1 else 2", "if . 1 end", "@", "$", "!", ".[", ".[1",
		"{(1): 2}", "{(true): 2}", "{([1]): 2}", "{({}): 2}", "{(1 == 1): 2}", "{(. | 1): 2}", "empty | {(null): 1}",
		"and", "or", "then", "end", `test("(")`, `test("*a")`, `test("^*")`, `test("(?:^)?")`, `test("\\z+")`,
		strings.Repeat("(", 2000) + "." + strings.Repeat(")", 2000),
		strings.Repeat(`"\(`, 2000) + "1" + strings.Repeat(`)"`, 2000),
	} {
		t.Run(filter, func(t *testing.T) {
			q, err := Parse(filter)
			if err == nil {
				t.Fatalf("Parse(%q) = %v, want an error", filter, q)
			}
			var u *UnsupportedError
			if errors.As(err, &u) {
				t.Fatalf("Parse(%q) = %v, want a syntax error, not an unsupported construct", filter, err)
			}
		})
	}
}

func TestDecimal(t *testing.T) {
	for _, tc := range []struct{ in, canonical string }{
		{"0", "0"}, {"-0", "-0"}, {"0.0", "0.0"}, {"-0.0", "-0.0"}, {"00", "0"}, {"01", "1"},
		{"1", "1"}, {"1.0", "1.0"}, {"1.50", "1.50"}, {"1.", "1"}, {".5", "0.5"},
		{"1e2", "1E+2"}, {"1E2", "1E+2"}, {"1e+2", "1E+2"}, {"1e-0", "1"}, {"10e-1", "1.0"},
		{"1e-5", "0.00001"}, {"0.000001", "0.000001"}, {"0.0000001", "1E-7"}, {"1.5e-7", "1.5E-7"},
		{"0e3", "0E+3"}, {"1e1000", "1E+1000"}, {"123.4560", "123.4560"}, {"5e-1", "0.5"}, {"50e-1", "5.0"},
		{"100000000000000000001", "100000000000000000001"},
		{"1.23456789012345678901234567890123456789e5", "123456.789012345678901234567890123456789"},
	} {
		d, ok := parseDecimal(tc.in)
		if !ok {
			t.Errorf("parseDecimal(%q) failed", tc.in)
			continue
		}
		if got := d.String(); got != tc.canonical {
			t.Errorf("parseDecimal(%q).String() = %q, want %q", tc.in, got, tc.canonical)
		}
	}
	for _, bad := range []string{"", "-", ".", "e5", "1e", "1e+", "1.2.3", "1x", "--1", "1e9999999999"} {
		if _, ok := parseDecimal(bad); ok {
			t.Errorf("parseDecimal(%q) succeeded", bad)
		}
	}
	for _, tc := range []struct {
		in   string
		want float64
	}{
		{"1.5", 1.5}, {"-2", -2}, {"1e1000", math.Inf(1)}, {"1e-1000", 0},
		// jq rounds to 17 significant digits (half up) before converting.
		{"12345678901234567890123456789012345678901234567890", 1.2345678901234568e49},
		{"1.00000000000000011102230246251565404236316680908203125", 1},
	} {
		d, _ := parseDecimal(tc.in)
		if got := d.float(); got != tc.want {
			t.Errorf("float(%s) = %v, want %v", tc.in, got, tc.want)
		}
	}
	cmp := func(a, b string) int {
		x, _ := parseDecimal(a)
		y, _ := parseDecimal(b)
		return cmpDecimal(x, y)
	}
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"1", "1.0", 0}, {"0", "-0", 0}, {"1e2", "100", 0}, {"0.1", "0.10", 0},
		{"100000000000000000001", "100000000000000000000", 1}, {"-1", "1", -1}, {"-2", "-1", -1},
		{"1e1000", "1e999", 1}, {"0.001", "0.01", -1}, {"9.99", "10", -1}, {"-0.5", "0", -1},
	} {
		if got := cmp(tc.a, tc.b); got != tc.want {
			t.Errorf("cmpDecimal(%s, %s) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

// pointThree is 0.1 + 0.2 computed in float64, not as an exact constant.
var pointOne, pointTwo = 0.1, 0.2
var pointThree = pointOne + pointTwo

func TestFormatDouble(t *testing.T) {
	for _, tc := range []struct {
		f    float64
		want string
	}{
		{0, "0"}, {math.Copysign(0, -1), "-0"}, {1, "1"}, {-1.5, "-1.5"}, {100, "100"},
		{1e15, "1000000000000000"}, {1e16, "1e+16"}, {1e17, "1e+17"}, {123456789012345678, "123456789012345680"},
		{0.0001, "0.0001"}, {0.00001, "1e-05"}, {1e-7, "1e-07"}, {pointThree, "0.30000000000000004"},
		{1.2345678901234567e49, "1.2345678901234567e+49"}, {1e100, "1e+100"}, {1e308, "1e+308"},
		{math.Inf(1), "1.7976931348623157e+308"}, {math.Inf(-1), "-1.7976931348623157e+308"}, {math.NaN(), "null"},
	} {
		if got := formatDouble(tc.f); got != tc.want {
			t.Errorf("formatDouble(%v) = %q, want %q", tc.f, got, tc.want)
		}
	}
}

func TestDecode(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`{"a":1,"b":2,"a":3}`, "{\n  \"a\": 3,\n  \"b\": 2\n}\n"},
		{`"\ud83d\ude00 \ud83d x \u00e9"`, "😀 \ufffd x é\n"},
		{"\"\xff\xfe\"", "\ufffd\ufffd\n"},
		{" [ 1 , 2 ] \n", "[\n  1,\n  2\n]\n"},
		{`[1.0, -0, 1e2]`, "[\n  1.0,\n  -0,\n  1E+2\n]\n"},
	} {
		got, err := run(t, ".", tc.in)
		if err != nil || got != tc.want {
			t.Errorf("decode %q = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
	for _, bad := range []string{"", "   ", "[1,]", "{a:1}", "01", "1.", ".5", "+1", "[1] 2", `"a`, "\"\t\"", "nul", "[", strings.Repeat("[", maxDepth+2)} {
		q, _ := Parse(".")
		if err := q.Write(&bytes.Buffer{}, []byte(bad)); err == nil {
			t.Errorf("decode %q succeeded", bad)
		}
	}
}

func TestCompileRegexEmits(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"a$", `a(?:\n?\z)`},
		{"^a|b$", `\Aa|b(?:\n?\z)`},
		{"(a|b)+", `(?:a|b)+`},
		{"(?<n>x)", `(?:x)`},
		{"[a-c\\]]", `[a-c\]]`},
		{"\\x41\\x{e9}", `\x41é`},
		{".é", ".é"},
		{"a{2,3}?", "a{2,3}?"},
		{"\\.\\/", `\.\/`},
	} {
		re, err := compileRegex(tc.in)
		if err != nil {
			t.Errorf("compileRegex(%q): %v", tc.in, err)
			continue
		}
		// \x41 is emitted as the character it stands for.
		want := strings.ReplaceAll(tc.want, `\x41`, "A")
		if got := re.String(); got != want {
			t.Errorf("compileRegex(%q) = %q, want %q", tc.in, got, want)
		}
	}
}

// A Query may be used from several goroutines (its regex cache is
// shared).
func TestConcurrentWrite(t *testing.T) {
	q, err := Parse(`.[] | test(.)`)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var b bytes.Buffer
			if err := q.Write(&b, []byte(`["a", "b+", "^c$", "a"]`)); err != nil {
				t.Error(err)
			}
			if b.String() != "true\ntrue\nfalse\ntrue\n" {
				t.Errorf("output %q", b.String())
			}
		}()
	}
	wg.Wait()
}

// Write stops at the first error the writer returns, and "?" does not
// swallow it.
func TestWriterError(t *testing.T) {
	q, err := Parse(`(.[] | .a)?`)
	if err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	err = q.Write(failingWriter{boom}, []byte(`[{"a":1},{"a":2}]`))
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the writer's error", err)
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func FuzzParse(f *testing.F) {
	for _, tc := range diffCases {
		f.Add(tc.filter)
	}
	for _, tc := range goldenCases {
		f.Add(tc.filter)
	}
	f.Fuzz(func(t *testing.T, filter string) {
		q, err := Parse(filter)
		if err != nil {
			return
		}
		var b limitedBuffer
		b.limit = 1 << 20
		_ = q.Write(&b, []byte(diffInput))
	})
}
