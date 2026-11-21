package filter

import (
	"encoding/json"
	"math/rand/v2"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2/internal/gcloudtest"
)

// treeScript writes gcloud's parse tree of each expression, with ANDs and
// ORs flattened as Node.Children flattens them, or the error that
// resource_filter.Compile raises.
const treeScript = `from googlecloudsdk.core.resource import resource_filter
OPS = {"_ExprHAS": ":", "_ExprEQ": "=", "_ExprNE": "!=", "_ExprLT": "<", "_ExprLE": "<=", "_ExprGT": ">", "_ExprGE": ">="}
def flat(n, cls, out):
    if type(n).__name__ == cls:
        flat(n._left, cls, out)
        flat(n._right, cls, out)
    else:
        out.append(dump(n))
    return out
def dump(n):
    if n is None:
        return {"none": True}
    c = type(n).__name__
    if c == "_ExprTRUE":
        return None
    if c in ("_ExprAND", "_ExprOR"):
        return {c[5:].lower(): flat(n, c, [])}
    if c == "_ExprNOT":
        return {"not": dump(n._expr)}
    if c not in OPS:
        return {"other": c}
    o = n._operand
    opd = o.string_value if o.list_value is None else [x.string_value for x in o.list_value]
    key = [k if not isinstance(k, float) else {"float": k} for k in n._key]
    return {"term": {"key": key, "op": OPS[c], "operand": opd, "transform": n._transform is not None}}
out = []
for e in json.load(sys.stdin):
    try:
        out.append({"tree": dump(resource_filter.Compile(e))})
    except Exception as ex:
        out.append({"error": type(ex).__name__ + ": " + str(ex)})
print(json.dumps(out))
`

type treeResult struct {
	Tree  any    `json:"tree"`
	Error string `json:"error"`
}

// gcloudShape returns n in treeScript's form, decoded from JSON as
// treeScript's output is.
func gcloudShape(n Node) any {
	var v any
	switch n.Kind() {
	case NoKind:
	case AndKind, OrKind:
		cs := []any{}
		for _, c := range n.Children() {
			cs = append(cs, gcloudShape(c))
		}
		v = map[string]any{map[Kind]string{AndKind: "and", OrKind: "or"}[n.Kind()]: cs}
	case NotKind:
		v = map[string]any{"not": gcloudShape(n.Children()[0])}
	case TermKind:
		key := []any{}
		for _, p := range n.Key() {
			switch {
			case p.Slice:
				key = append(key, nil)
			case p.Index != nil:
				key = append(key, json.Number(p.Index.String()))
			default:
				key = append(key, p.Name)
			}
		}
		var opd any = n.Operands()[0]
		if n.IsList() {
			opd = n.Operands()
		}
		v = map[string]any{"term": map[string]any{"key": key, "op": n.Op(), "operand": opd, "transform": false}}
	}
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		panic(err)
	}
	return out
}

// treeExprs are expressions on top of diffFilters for the tree tests:
// grouping and precedence, and the quoting and escaping String must get
// right for gcloud's lexer.
var treeExprs = []string{
	// AND, OR, NOT and juxtaposition
	"a=1 OR b=2 c=3", "a=1 b=2 OR c=3", "a=1 OR b=2 c=3 OR d=4", "a=1 OR b=2 OR c=3 d=4", "a=1 (b=2 OR c=3) OR d=4",
	"(a=1) (b=2) OR c=3", "a=1 OR (b=2 c=3)", "NOT a=1 OR b=2", "NOT a=1 b=2", "-a=1 -b=2 OR -c=3", "NOT -a=1", "NOT(a=1)",
	"(NOT a=1) AND (NOT b=2)", "a=1 AND b=2 AND (c=3 OR d=4 OR (e=5 AND f=6))", "((a=1 OR b=2) AND c=3) OR d=4",
	"(a=1)(b=2)", "a=1\tb=2", "a=1\nOR\nb=2", "a=1\u00a0b=2", " a=1 ", "a = 1", "a : x", "a:x b:y c:z OR d:w",
	// operands
	`a="x\"y"`, `a='x\'y'`, `a="x'y"`, `a='x"y'`, `a=x\"y`, `a=\\`, `a="\\\\"`, `a=x\\y`, `a="tab\there"`, "a=\"tab\there\"",
	"a=\"new\nline\"", `a=é`, `a="é ü"`, `a=日本`, `a="  lead"`, `a=""`, `a=''`, `a=x\ y`, "a=x\u3000y", `a=-1`, `a=--1`,
	`a: :x`, `a: =x`, `a: !x`, `a: <x`, `a: ~x`, `a=x:y`, `a=x=y`, `a=x<y`, `a=*`, `a:**`, `a:"*"`, `a:\*`, `a:'a*'`,
	`a=ANDb`, `a="OR"`, `a=AND`, `a=NOT`, `a=OR(`, `a=0x10`, `a=TRUE`,
	// lists
	`a=("")`, `a=(" ")`, `a=(x,"")`, `a:("a,b" c)`, `a:(x\,y)`, `a:(x\ y)`, `a:(OR x)`, `a:("OR" x)`, `a:(x OR y)`,
	`a:("(" ")")`, `a:(\( \))`, `a:((x))`, `a=(x,(y))`, `a = ( x , y )`, `a:(x,,y)`, `a:(x  y)`, `a:(,x)`, `a:(x,)`,
	// keys
	`"a b"=1`, `'a.b'.c=1`, `a."b c".d=1`, `a\.b=1`, `"a\\b"=1`, `"a\"b"=1`, `a[0]=1`, `a[-1]=1`, `a[ 1 ]=1`, `a[+1]=1`,
	`a[01]=1`, `a[1_0]=1`, `a[]=1`, `a[][0]=1`, `"[0]"=1`, `"a[0]"=1`, `"-a"=1`, `-"-a"=1`, `"NOTa"=1`, `NOTa=1`, `.=1`,
}

// treeCases returns each expression clix parses followed by its String
// form.
func treeCases(exprs []string) []string {
	var cases []string
	for _, e := range exprs {
		if f, err := Parse(e); err == nil {
			cases = append(cases, e, f.String())
		}
	}
	return cases
}

// checkTrees checks that clix parses each pair of cases, an expression
// and its String form, into gcloud's tree of the expression, and that
// gcloud parses the String form into the same tree.
func checkTrees(t *testing.T, cases []string, want []treeResult) {
	t.Helper()
	failures := 0
	for i := 0; i < len(cases); i += 2 {
		e, s := cases[i], cases[i+1]
		f, _ := Parse(e)
		got := gcloudShape(f.Root())
		switch {
		case want[i].Error != "":
			t.Errorf("%q: clix parses it as %v; gcloud fails: %s", e, got, want[i].Error)
		case !reflect.DeepEqual(got, want[i].Tree):
			t.Errorf("%q:\n clix   %v\n gcloud %v", e, got, want[i].Tree)
		case want[i+1].Error != "":
			t.Errorf("%q: gcloud fails on its String form %q: %s", e, s, want[i+1].Error)
		case !reflect.DeepEqual(want[i+1].Tree, want[i].Tree):
			t.Errorf("%q: gcloud parses its String form %q differently:\n %v\n %v", e, s, want[i].Tree, want[i+1].Tree)
		default:
			continue
		}
		if failures++; failures > 30 {
			t.Fatal("too many failures")
		}
	}
}

// TestTreeMatchesGcloud checks the trees of diffFilters and treeExprs,
// and of their String forms, against gcloud's, or without gcloud, its
// recorded trees (see gcloudtest.Golden).
func TestTreeMatchesGcloud(t *testing.T) {
	cases := treeCases(append(append([]string{}, diffFilters...), treeExprs...))
	var want []treeResult
	gcloudtest.Golden(t, "testdata/gcloud_tree.json", treeScript, cases, &want)
	checkTrees(t, cases, want)
}

// TestTreeRandomMatchesGcloud is TestTreeMatchesGcloud for random
// filters and random text full of quotes, escapes, operator characters
// and logical words. CLIX_GCLOUD_CASES sets the number of filters of
// each kind (default 3000) and CLIX_GCLOUD_SEED the seed.
func TestTreeRandomMatchesGcloud(t *testing.T) {
	if os.Getenv("CLIX_GCLOUD_SDK") == "" {
		t.Skip("set CLIX_GCLOUD_SDK to compare with gcloud")
	}
	n := 3000
	if s := os.Getenv("CLIX_GCLOUD_CASES"); s != "" {
		n, _ = strconv.Atoi(s)
	}
	seed := uint64(1)
	if s := os.Getenv("CLIX_GCLOUD_SEED"); s != "" {
		seed, _ = strconv.ParseUint(s, 10, 64)
	}
	rng := rand.New(rand.NewPCG(seed, seed))
	g := &gen{rng: rng}
	runes := []rune(`abANDORT "'\(),:=!<>~*-.[]` + "\t\né01")
	text := func(max int) string {
		var b strings.Builder
		for range rng.IntN(max + 1) {
			b.WriteRune(runes[rng.IntN(len(runes))])
		}
		return b.String()
	}
	ops := []string{":", "=", "!=", "<", "<=", ">", ">=", " : ", "= "}
	var exprs []string
	for range n {
		exprs = append(exprs, g.expr(0))
		key := "k"
		if rng.IntN(2) == 0 {
			key = "k" + text(3)
		}
		e := key + ops[rng.IntN(len(ops))] + text(6)
		if rng.IntN(3) == 0 {
			e += " " + text(5)
		}
		exprs = append(exprs, e)
	}
	cases := treeCases(exprs)
	var want []treeResult
	gcloudtest.Run(t, treeScript, cases, &want)
	checkTrees(t, cases, want)
	t.Logf("%d filters (seed %d), %d parsed by clix and compared with gcloud", len(exprs), seed, len(cases)/2)
}
