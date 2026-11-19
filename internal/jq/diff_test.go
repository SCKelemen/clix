package jq

import (
	"strings"
	"testing"
)

// diffInput is shaped like clix.FormatData's JSON output: struct fields in
// declared order, map keys sorted, two-space indent.
const diffInput = `{
  "name": "widget",
  "displayName": "Widget \"One\"",
  "count": 3,
  "price": 1.5,
  "zero": 0,
  "negative": -2,
  "big": 9007199254740993,
  "huge": 100000000000000000001,
  "ratio": 1e-7,
  "large": 1e+21,
  "tags": [
    "a",
    "B",
    "c d"
  ],
  "owner": {
    "admin": true,
    "id": 42,
    "login": "sam"
  },
  "items": [
    {
      "id": 1,
      "name": "x",
      "ok": true,
      "score": 2.5
    },
    {
      "id": 2,
      "name": "y",
      "ok": false,
      "score": 10
    },
    {
      "id": 3,
      "name": null,
      "ok": null,
      "score": 1.5
    }
  ],
  "empty": [],
  "none": null,
  "obj": {},
  "text": "line1\nline2\ttab\u007f\u001f é😀<>&",
  "flag": false
}
`

type diffCase struct {
	filter   string
	input    string // diffInput when empty
	literal  bool   // depends on jq keeping number literals (decNumber)
	rejected bool   // clix fails with "not supported in clix --jq" (jq may not)
}

var diffCases = []diffCase{
	// Paths.
	{filter: "."},
	{filter: ""},
	{filter: ".name"},
	{filter: ".owner.login"},
	{filter: `."displayName"`},
	{filter: `.["owner"]["id"]`},
	{filter: `.owner."login"`},
	{filter: ".items[0]"},
	{filter: ".items[0].name"},
	{filter: ".items[-1]"},
	{filter: ".items[-4]"},
	{filter: ".items[3]"},
	{filter: ".items[1.7]"},
	{filter: ".items[-1.5]"},
	{filter: ".items[]"},
	{filter: ".items[].name"},
	{filter: ".items[]?"},
	{filter: ".name[]?"},
	{filter: ".name[]"},
	{filter: ".name.x"},
	{filter: ".name.x?"},
	{filter: ".missing"},
	{filter: ".missing.deeper"},
	{filter: ".none[0]"},
	{filter: ".none[]"},
	{filter: ".none[]?"},
	{filter: ".tags[0,2]"},
	{filter: `.owner["id","login"]`},
	{filter: ".items[0,1].id"},
	{filter: ".items[.count]"},
	{filter: ".items.[0]"},
	{filter: ". .name"},
	{filter: ".items[0]?.name"},
	{filter: ".a?.b"},
	{filter: "[1,{\"a\":2},3] | [.[].a?], [(.[].a)?], [.[][]?], [.[][\"a\"]?]"},
	{filter: "[.name.x?, 5], [.tags[].x?], [.tags[][]?], [(.tags[].x)?]"},
	{filter: ".tags[]?[]?"},
	{filter: ".tags | .[\"a\"]"},
	{filter: ".owner | .[0]"},
	{filter: ".owner[null]"},
	{filter: ".tags[true]"},
	{filter: ".none[true]"},
	{filter: "[..]"},
	{filter: "[..?]"},
	{filter: `[.. | select(type == "number")]`},
	{filter: ".owner | ..[0]?"},
	{filter: `."\(.name)"`},
	{filter: ".[\"a\" , \"b\"]", input: `{"a":1,"b":2}`},
	{filter: ".[]", input: `{"b":1,"a":2}`},
	{filter: ".", input: `{"a":1,"b":2,"a":3}`},
	{filter: "keys_unsorted", input: `{"a":1,"b":2,"a":3}`},

	// Pipes, comma, parentheses, try.
	{filter: ".items | .[0] | .id"},
	{filter: ".name, .count, .tags"},
	{filter: "(.name, .count) | tostring"},
	{filter: "[.items[] | .id, .name]"},
	{filter: "(1,2)?"},
	{filter: "[(1, (1|keys), 2)?]"},
	{filter: "[(.tags[], (1|keys))?]"},
	{filter: "[(1,2)? | keys]"},
	{filter: "[.items[]? | keys]"},
	{filter: ".a??"},
	{filter: "1??"},
	{filter: "if true then 1 else 2 end?"},
	{filter: "(1)?.b"},
	{filter: "..?[0]"},

	// Literals.
	{filter: "null, true, false"},
	{filter: "1, 1.5, 100, 0.1, 123456789"},
	{filter: "1.0, 1.50, 1e2, 1E2, 1e-5, 1.5e-7, 0.0, 00, 01, 1., .5, 10e-1", literal: true},
	{filter: "1e1000, 100000000000000000001, 1e17, 123456789012345678", literal: true},
	{filter: "-1, -1.0, -0, - 1, -1e2, -0.0"},
	{filter: "[-1, 2, -3.5]"},
	{filter: `"plain", "esc\"aped\\ \/ \b\f\n\r\t", "é😀", "\u007f\u0001"`},
	{filter: `"a\(1,2)b\(3,4)"`},
	{filter: `"\(.name) has \(.count) \(.tags)"`},
	{filter: `"\(1.0) \(null) \(true) \({"a":[1]})"`, literal: true},
	{filter: `"\("x")"`},
	{filter: `"\(.items[].id)-\(.tags[])"`},
	{filter: "# comment\n.name # trailing"},
	{filter: "[1, # inside\n 2]"},

	// Comparisons.
	{filter: ".count == 3, .count != 3, .count < 4, .count <= 3, .count > 3, .count >= 3"},
	{filter: "[(1,2) < (3,0)]"},
	{filter: "[(1,2) == (3,1)]"},
	{filter: ".big == 9007199254740992, .big == 9007199254740993, .big > 9007199254740992", literal: true},
	{filter: ".huge == 100000000000000000000, .huge > 100000000000000000000", literal: true},
	{filter: "1.0 == 1, 1 == 1.00, 1e2 == 100, 0.1 == 0.10"},
	{filter: "1e1000 == 1e1001, 1e1000 > 1e999", literal: true},
	{filter: "null < false, false < true, true < 0, 0 < \"\", \"\" < [], [] < {}"},
	{filter: `"a" < "b", "B" < "a", "é" > "z", "ab" < "abc"`},
	{filter: "[1,2] < [1,3], [1,2] < [1,2,0], [2] > [1,9]"},
	{filter: `{"a":1} < {"b":0}, {"a":2} > {"a":1}, {"a":1} < {"a":1,"b":0}, {"b":1,"a":2} == {"a":2,"b":1}`},
	{filter: ".items[0] == .items[0], .items[0] == .items[1]"},
	{filter: "(.count | length) == 3, -1 < 0, -0 == 0"},
	{filter: ". == . == ."},

	// and, or, not, //, if.
	{filter: "true and false, true or false, null and true, null or 1, 1 and 1"},
	{filter: "[(true,false) and (true,false)]"},
	{filter: "[(false,true) or (false,true)]"},
	{filter: "[.items[].ok | not]"},
	{filter: "not"},
	{filter: ".none // \"default\""},
	{filter: ".flag // .count // 0"},
	{filter: "[(1,2) // (3,4)], [(null,false) // (3,4)], [(null,1,false,2) // 3], [empty // 3]"},
	{filter: "[(1|keys) // 3]"},
	{filter: "[false // false // 3]"},
	{filter: ".items[] | if .ok then \"yes\" elif .ok == false then \"no\" else \"unknown\" end"},
	{filter: "[if (true,false) then 1 else 2 end]"},
	{filter: "[if (true,false) then (1,2) end]"},
	{filter: "if . then 1 elif . then 2 end"},
	{filter: "1 == if true then 1 else 2 end"},
	{filter: "if true then {\"a\":1} else 2 end.a"},

	// Construction.
	{filter: "[]"},
	{filter: "[.items[].id]"},
	{filter: "{}"},
	{filter: "{name, count}"},
	{filter: "{n: .name, c: .count, t: .tags[0]}"},
	{filter: `{"name": .name, "x\(.count)": 1}`},
	{filter: "{(.name): .count}"},
	{filter: "{(.tags[]): 1}"},
	{filter: "{a: (1,2), b: (3,4)}"},
	{filter: `{("a","b"): (1,2)}`},
	{filter: `{"\(1,2)"}`},
	{filter: "{if: 1, then: 2, and: 3}"},
	{filter: "{if}"},
	{filter: "{a: .owner | .login}"},
	{filter: "{a: -1}"},
	{filter: "{a: -1 | 2}"},
	{filter: "{a: 1, b: 2,}"},
	{filter: "{a: .x == 1}"},
	{filter: "{a: 1?}"},
	{filter: "{a: .x // 1}"},
	{filter: "{a: if true then 1 else 2 end}"},
	{filter: "{(1): 2}"},
	{filter: "{(.count): 2}"},
	{filter: "{a: 1}.a, [1][0], {\"a\":1}[\"a\"]"},
	{filter: "[1,]"},
	{filter: "[.items[] | {id, name}]"},

	// Builtins.
	{filter: ".items | map(.id)"},
	{filter: ".owner | map(.)"},
	{filter: ".name | map(.)"},
	{filter: "[.items[] | select(.ok)]"},
	{filter: "[.items[] | select(.id > 1) | .name]"},
	{filter: "[.items[] | select((.id > 1), (.id > 2)) | .id]"},
	{filter: "keys, (.owner | keys), (.tags | keys)"},
	{filter: ".name | keys"},
	{filter: "null | keys"},
	{filter: `{"b":1,"a":2,"A":0,"é":1,"z":3} | keys, keys_unsorted`},
	{filter: "[.[] | length?]"},
	{filter: ".name, .tags, .owner, .none, .count, .negative, .price | length"},
	{filter: ".flag | length"},
	{filter: ".text | length"},
	{filter: "-1.5, 1e1000, -1e1000, 123456789012345678, 1.2345678901234567890123e49 | length", literal: true},
	{filter: ".owner | has(\"id\", \"x\")"},
	{filter: ".tags | has(0, 2, 3, -1, 1.5, -0.5)"},
	{filter: ".tags | has(\"a\")"},
	{filter: ".owner | has(0)"},
	{filter: "null | has(\"a\")"},
	{filter: "null | has(0)"},
	{filter: ".name | has(0)"},
	{filter: ".owner | to_entries"},
	{filter: ".tags | to_entries"},
	{filter: ".none | to_entries"},
	{filter: ".obj | to_entries"},
	{filter: ".tags | first, last"},
	{filter: ".empty | first, last"},
	{filter: ".none | first, last"},
	{filter: ".name | first"},
	{filter: ".owner | last"},
	{filter: "first(.items[].id), last(.items[].id)"},
	{filter: "[first(empty)], [last(empty)]"},
	{filter: "first(1, (1|keys))"},
	{filter: "[last(1, (1|keys))]"},
	{filter: "first((1,2) | keys)"},
	{filter: ".tags | join(\", \")"},
	{filter: "[1, null, \"a\", true, 1.0] | join(\"-\")", literal: true},
	{filter: "[[1]] | join(\",\")"},
	{filter: "[{\"a\":1}] | join(\",\")"},
	{filter: "[null] | join(\",\"), ([null,null] | join(\",\")), ([] | join(\",\"))"},
	{filter: "[\"a\",\"b\"] | join(1)"},
	{filter: "[\"a\"] | join(1), ([1] | join(1)), ([\"a\",\"b\"] | join(null))"},
	{filter: "[1,2] | join(\",\", \";\")"},
	{filter: ".owner | join(\",\")"},
	{filter: ".name | join(\",\")"},
	{filter: ".none | join(\",\")"},
	{filter: "[1e2, 100000000000000000001] | join(\",\")", literal: true},
	{filter: ".count, .tags, .owner, .name, .none, .price | tostring"},
	{filter: "1.0, 1e2 | tostring, tojson", literal: true},
	{filter: ". | tojson"},
	{filter: ".text | tojson"},
	{filter: ".displayName | ascii_downcase, ascii_upcase"},
	{filter: `"ABC é Ä xyz" | ascii_downcase, ascii_upcase`},
	{filter: ".count | ascii_downcase"},
	{filter: "[.[] | type]"},
	{filter: "[.items[].name | values]"},
	{filter: "[.items[].ok | values]"},
	{filter: "[empty], [1, empty, 2]"},
	{filter: ".tags | sort"},
	{filter: "[3,1,null,\"a\",true,false,[1],{\"a\":1},{\"a\":0,\"b\":1},1.0,1] | sort", literal: true},
	{filter: ".name | sort"},
	{filter: ".none | sort"},
	{filter: ".items | sort_by(.score)"},
	{filter: ".items | sort_by(.name)"},
	{filter: ".items | sort_by(.ok, .id)"},
	{filter: "[{\"a\":[2]},{\"a\":[1,5]}] | sort_by(.a[])"},
	{filter: ".items | sort_by(empty)"},
	{filter: ".owner | sort_by(.)"},
	{filter: ".name | sort_by(.)"},

	// Regex.
	{filter: ".tags[] | test(\"a\")"},
	{filter: ".name | test(\"^wid\"), test(\"get$\"), test(\"^get\"), test(\"\"), test(\"W\")"},
	{filter: `"a\n" | test("a$"), test("a\n$"), test("^a$")`},
	{filter: `"a\nb" | test("a$"), test("^b"), test("a.b"), test("b$")`},
	{filter: `"abc" | test("b"), test("x"), test("a|x"), test("^(ab|x)c$"), test("(?:a)(?<n>b)c")`},
	{filter: `"aaa" | test("a{3}"), test("^a{4}"), test("a{2,}"), test("^a{1,2}$"), test("a+?"), test("^a*?$")`},
	{filter: `"é😀" | test("é"), test("^.{2}$"), test("\\p{L}"), test("\\p{So}"), test("^\\P{L}")`},
	{filter: `"Ab9_" | test("^[A-Za-z0-9_]+$"), test("[^a-z]"), test("[a\\-z]"), test("[-z]"), test("[z-]")`},
	{filter: `"a.b" | test("a\\.b"), test("a[.]b"), test("\\x2e"), test("\\x{2E}"), test("a\\/b")`},
	{filter: `"tab\there" | test("\\t"), test("[\\t]"), test("\\n")`},
	{filter: `"Ελληνικά" | test("\\p{Greek}"), test("^\\p{Latin}")`},
	{filter: `"x" | test("x", "y")`},
	{filter: `"ab" | test(["a"]), test(["x"])`},
	{filter: `"ab" | test(["a", null]), test(["a", ""]), test(["a", null, 1])`},
	{filter: `"ab" | test(["a", "x"])`, rejected: true},
	{filter: `"a" | test("\\d")`, rejected: true},
	{filter: `"a" | test("\\d" | ascii_downcase)`, rejected: true},
	{filter: `[1,2,1] | .[[1]], .[[]], .[[1,2]], .[[3]], .[[2,1]]`},
	{filter: `[1,1,1] | .[[1,1]]`},
	{filter: `[[1],1.0,1] | .[[[1]]], .[[1]]`},
	{filter: `"abc" | .[[1]]`},
	{filter: `null | .[[1]]`},
	{filter: `null | .[{}]`},
	{filter: `{(1): empty}`},
	{filter: `{("a",1): (1,2)}`},
	{filter: `{(-1): 2}`},
	{filter: `{([-1]): 2}`},
	{filter: `{([1,[2],{"a":null}]): 2}`},
	{filter: `{({a: 1}): 2}`},
	{filter: `{({a}): 2}`},
	{filter: `empty | {(null): 2}`},
	{filter: `{((("a"))): 2}`},
	{filter: `{([] | .): 1}`},
	{filter: `{(. | . | 1): 2}`},
	{filter: `{([.|1]): 2}`},
	{filter: `{(.|"a"): 2}`},
	{filter: `{(1 == 1): 2}`},
	{filter: `{([1 == 1]): 2}`},
	{filter: `{(1?): 2}`},
	{filter: `{(1 // 2): 2}`},
	{filter: `{(if true then 1 else 2 end): 2}`},
	{filter: `{(empty): 2}`},
	{filter: `["\u0000a\u0001\u001f\u007f", 1] | @tsv, @csv`},
	{filter: `[1,2,3] | .[{"start":1,"end":2}]`, rejected: true},
	{filter: `.[1:]`, rejected: true},
	{filter: `.count + 1`, rejected: true},
	{filter: `.a = 1`, rejected: true},
	{filter: `. as $x | $x`, rejected: true},
	{filter: `[limit(1; .[])]`, rejected: true},
	{filter: `1 | test("a")`},
	{filter: `"a" | test(1)`},
	{filter: `"a" | test(null)`},
	{filter: `"a" | test([])`},
	{filter: `.tags | map(test("^[a-z]"))`},
	{filter: `[.tags[] | select(test("^[a-z]$"))]`},
	{filter: `.items | map(select(.name | values | test("x")))`},

	// Formats.
	{filter: "[.name, .count, .none, .flag, .price] | @tsv, @csv"},
	{filter: "[1.0, 1e2, 100000000000000000001, -0] | @tsv, @csv", literal: true},
	{filter: `["a\tb", "c\\d", "e\rf", "g\nh", "q\"x"] | @tsv, @csv`},
	{filter: "[] | @tsv, @csv"},
	{filter: "[[1]] | @tsv"},
	{filter: "[{\"a\":1}] | @csv"},
	{filter: ".name | @tsv"},
	{filter: ".name | @csv"},
	{filter: ".owner | @json, @text"},
	{filter: ".name | @json, @text"},
	{filter: ".none | @text"},
	{filter: ".items[] | [.id, .name] | @tsv"},

	// Runtime errors after some output.
	{filter: ".tags[], (.name | keys)"},
	{filter: ".items[] | .name | ascii_downcase"},
	{filter: "(.count | keys), 1"},

	// Other inputs.
	{filter: ".", input: "[]"},
	{filter: ".", input: "[1,[2,[3]],{\"a\":{}}]"},
	{filter: ".[] | length", input: `["", "abc", [1,2], {"a":1}, null, -3]`},
	{filter: ".", input: `"just a string"`},
	{filter: ".", input: `"tab\there\u0000"`},
	{filter: "., length, tostring", input: "1.000", literal: true},
	{filter: ".", input: "-0"},
	{filter: ".[0] == .[1], .[0] < .[1]", input: "[12345678901234567890123456789012345678901234567890, 12345678901234567890123456789012345678901234567891]", literal: true},
	{filter: ".", input: "[0.1, 1e-7, 1.5e300, 5e-1, 50e-1, 0.5e1, 1E+0, 0e3]", literal: true},
	{filter: "map(length)", input: "[0.1, 1e-7, 1.5e300, 5e-1, 123456.789012345678901234567890123456789]", literal: true},
	{filter: ".", input: `{"a":"😀","b":"é"}`},
}

// withoutLiterals replaces the numbers of diffInput that a jq built without
// decNumber prints differently (it keeps only a double).
var withoutLiterals = strings.NewReplacer("9007199254740993", "9007199254740991", "100000000000000000001", "12345", "1e-7", "0.5", "1e+21", "1000")

func TestDifferentialTable(t *testing.T) {
	bin := requireJQ(t)
	t.Logf("comparing with %s (decNumber: %v) at %s", bin.version, bin.decNum, bin.path)
	for _, tc := range diffCases {
		t.Run(strings.ReplaceAll(tc.filter, "/", "∕"), func(t *testing.T) {
			t.Parallel()
			if tc.literal && !bin.decNum {
				t.Skip("jq was built without decNumber")
			}
			input := tc.input
			if input == "" {
				input = diffInput
				if !bin.decNum {
					input = withoutLiterals.Replace(input)
				}
			}
			if tc.rejected {
				_, parseErr, runErr := runClix(tc.filter, input)
				err := parseErr
				if err == nil {
					err = runErr
				}
				if err == nil || !strings.Contains(err.Error(), "not supported in clix --jq") {
					t.Fatalf("filter %q: err = %v, want a \"not supported in clix --jq\" error", tc.filter, err)
				}
				return
			}
			compareWithJQ(t, bin, tc.filter, input)
		})
	}
}

// compareWithJQ fails t when Query.Write and jq -r disagree on filter and
// input: in stdout, or in whether they fail.
func compareWithJQ(t *testing.T, bin jqBinary, filter, input string) {
	t.Helper()
	want, jqOK, jqErr := runJQ(t, bin, filter, input)
	got, parseErr, runErr := runClix(filter, input)
	if parseErr != nil {
		if jqOK {
			t.Fatalf("filter %q: Parse failed but jq succeeded\nParse: %v\njq stdout: %q", filter, parseErr, want)
		}
		return
	}
	if got != want {
		t.Fatalf("filter %q: output differs\nclix: %q\njq:   %q\nclix error: %v\njq stderr: %s", filter, got, want, runErr, jqErr)
	}
	if (runErr == nil) != jqOK {
		t.Fatalf("filter %q: clix error %v, jq ok %v (stderr %q)", filter, runErr, jqOK, jqErr)
	}
}

// The golden cases, which run without jq, must agree with jq too.
func TestGoldenMatchesJQ(t *testing.T) {
	bin := requireJQ(t)
	if !bin.decNum {
		t.Skip("the golden cases print number literals; jq was built without decNumber")
	}
	for _, tc := range goldenCases {
		if _, err := Parse(tc.filter); err != nil {
			continue
		}
		if strings.Contains(tc.wantErr, "not supported in clix --jq") || strings.HasPrefix(tc.wantErr, "regex") {
			continue // clix refuses what jq runs
		}
		t.Run(tc.filter, func(t *testing.T) {
			t.Parallel()
			compareWithJQ(t, bin, tc.filter, tc.input)
		})
	}
}
