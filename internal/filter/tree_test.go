package filter

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2/internal/resource"
)

// shape writes n's tree as kind(children...) with terms as key op [operands],
// keys as gcloud's GetKeyName writes them, quoting nothing.
func shape(n Node) string {
	switch n.Kind() {
	case NoKind:
		return "nil"
	case TermKind:
		list := ""
		if n.IsList() {
			list = " list"
		}
		return fmt.Sprintf("%s %s %q%s", n.Key(), n.Op(), n.Operands(), list)
	}
	var parts []string
	for _, c := range n.Children() {
		parts = append(parts, shape(c))
	}
	name := map[Kind]string{AndKind: "and", OrKind: "or", NotKind: "not"}[n.Kind()]
	return name + "(" + strings.Join(parts, ", ") + ")"
}

func TestTree(t *testing.T) {
	for _, tt := range []struct{ expr, shape, str string }{
		{"", "nil", ""},
		{"()", "nil", ""},
		{"a=1", `a = ["1"]`, "a=1"},
		{"a:(x y)", `a : ["x" "y"] list`, "a:(x,y)"},
		{"a=(x)", `a = ["x"] list`, "a=(x)"},
		{"-a:*", `not(a : ["*"])`, "NOT a:*"},
		{"NOT a<2", `not(a < ["2"])`, "NOT a<2"},
		{"NOT -a<2", `not(not(a < ["2"]))`, "NOT (NOT a<2)"},
		{"a=1 b=2 c=3", `and(a = ["1"], b = ["2"], c = ["3"])`, "a=1 AND b=2 AND c=3"},
		{"a=1 AND (b=2 AND c=3)", `and(a = ["1"], b = ["2"], c = ["3"])`, "a=1 AND b=2 AND c=3"},
		{"(a=1 AND b=2) c=3", `and(a = ["1"], b = ["2"], c = ["3"])`, "a=1 AND b=2 AND c=3"},
		{"a=1 OR b=2 OR c=3", `or(a = ["1"], b = ["2"], c = ["3"])`, "a=1 OR b=2 OR c=3"},
		// Juxtaposition is below OR: a b OR c is a AND (b OR c).
		{"a=1 b=2 OR c=3", `and(a = ["1"], or(b = ["2"], c = ["3"]))`, "a=1 AND (b=2 OR c=3)"},
		{"(a=1 AND b=2) OR c=3", `or(and(a = ["1"], b = ["2"]), c = ["3"])`, "(a=1 AND b=2) OR c=3"},
		{"NOT (a=1 OR b=2)", `not(or(a = ["1"], b = ["2"]))`, "NOT (a=1 OR b=2)"},
		{"((a=1))", `a = ["1"]`, "a=1"},
		{"labels.env:prod disks[0].name=boot tags.items[]:web", `and(labels.env : ["prod"], disks[0].name = ["boot"], tags.items[] : ["web"])`,
			"labels.env:prod AND disks[0].name=boot AND tags.items[]:web"},
		{`"k.e.y".value=1`, `k.e.y.value = ["1"]`, `"k.e.y".value=1`},
		{`name:'a b' x="say \"hi\""`, `and(name : ["a b"], x = ["say \"hi\""])`, `name:"a b" AND x="say \"hi\""`},
		{`name="" x=("" a)`, `and(name = [""], x = ["a"] list)`, `name="" AND x=(a)`},
		{`x=("",a)`, `x = ["" "a"] list`, `x=("",a)`},
		{`a="AND" b='=c' c=a,b d=(a,"b c")`, `and(a = ["AND"], b = ["=c"], c = ["a,b"], d = ["a" "b c"] list)`,
			`a="AND" AND b="=c" AND c="a,b" AND d=(a,"b c")`},
		{`.:x`, ` : ["x"]`, ".:x"}, // the empty key, the resource itself
		{`"-a"=1`, `-a = ["1"]`, `"-a"=1`},
		{`a\\b=c`, `a\b = ["c"]`, `"a\\b"=c`},
	} {
		f, err := Parse(tt.expr)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.expr, err)
			continue
		}
		if got := shape(f.Root()); got != tt.shape {
			t.Errorf("Parse(%q) is\n %s\nwant\n %s", tt.expr, got, tt.shape)
		}
		if got := f.String(); got != tt.str {
			t.Errorf("Parse(%q).String() = %s, want %s", tt.expr, got, tt.str)
		}
	}
}

// checkRoundTrip checks that expr, if it parses, prints as an expression
// that parses into the same tree and prints the same, and that both
// select the same resources of docs.
func checkRoundTrip(t *testing.T, expr string, docs [][]any) {
	t.Helper()
	f, err := Parse(expr, WithLocation(time.UTC))
	if err != nil {
		return
	}
	s := f.String()
	g, err := Parse(s, WithLocation(time.UTC))
	if err != nil {
		t.Fatalf("Parse(%q).String() = %q, which does not parse: %v", expr, s, err)
	}
	if a, b := shape(f.Root()), shape(g.Root()); a != b {
		t.Fatalf("Parse(%q) is %s, but Parse(%q) is %s", expr, a, s, b)
	}
	if s2 := g.String(); s2 != s {
		t.Fatalf("String is not stable: %q, then %q", s, s2)
	}
	for _, doc := range docs {
		a, errA := f.Select(doc)
		b, errB := g.Select(doc)
		if fmt.Sprint(a, errA) != fmt.Sprint(b, errB) {
			t.Fatalf("%q selects %v, %v; %q selects %v, %v", expr, a, errA, s, b, errB)
		}
	}
}

func decodedDocs(t testing.TB) [][]any {
	t.Helper()
	docs := make([][]any, len(diffDocs))
	for i, d := range diffDocs {
		v, err := resource.Decode([]byte(d))
		if err != nil {
			t.Fatal(err)
		}
		docs[i] = v.([]any)
	}
	return docs
}

func TestStringRoundTrip(t *testing.T) {
	docs := decodedDocs(t)
	exprs := append([]string{}, diffFilters...)
	exprs = append(exprs,
		`"a b".c:'x y'`, `"a\"b"=1`, `'it''s'=x`, `a.b[0][-1].c=1`, `a[]:x`, `[0]=1`, `a="NOT"`, `a:OR*`, `a=(x,"y z",'')`,
		`a=:x`, `a<"<b>"`, "a=x y", "a='line\nbreak'", `a:(x(y) z)`, `a='('`, `a=\'x`, `a="\\"`, `a:"*"`)
	g := &gen{rng: rand.New(rand.NewPCG(2, 2))}
	for range 3000 {
		exprs = append(exprs, g.expr(0))
	}
	for _, e := range exprs {
		checkRoundTrip(t, e, docs)
	}
}

func FuzzString(f *testing.F) {
	for _, s := range append(append([]string{}, diffFilters...), `"a b".c:'x y'`, `a=(x,"y z",'')`, "NOT -a=1", `a="\\"`) {
		f.Add(s)
	}
	docs := decodedDocs(f)
	f.Fuzz(func(t *testing.T, expr string) {
		checkRoundTrip(t, expr, docs)
	})
}

func TestSub(t *testing.T) {
	docs := decodedDocs(t)
	f, err := Parse("name:vm status=RUNNING (labels.env=prod OR name:db)", WithLocation(time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	c := f.Root().Children()
	if len(c) != 3 {
		t.Fatalf("children %d", len(c))
	}
	if s := f.Sub().String(); s != "" {
		t.Errorf("Sub() = %q", s)
	}
	if s := f.Sub(c[2], Node{}, c[0]).String(); s != "(labels.env=prod OR name:db) AND name:vm" {
		t.Errorf("Sub(c2, zero, c0) = %q", s)
	}
	// The conjunction of the parts selects what the whole does.
	whole, _ := f.Select(docs[2])
	parts, _ := f.Sub(c...).Select(docs[2])
	if fmt.Sprint(whole) != fmt.Sprint(parts) || fmt.Sprint(whole) != "[0]" {
		t.Errorf("Select = %v, Sub(all).Select = %v", whole, parts)
	}
}

// A Selector given a list in parts selects what Select of the whole list
// does, including the date and time state a term keeps for the list.
func TestSelector(t *testing.T) {
	doc := diffDocs[4] // "pending" first: t compares text for the whole list
	v, _ := resource.Decode([]byte(doc))
	list := v.([]any)
	f, err := Parse("t<2018-01-01", WithLocation(time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	whole, err := f.Select(list)
	if err != nil {
		t.Fatal(err)
	}
	s := f.Selector()
	var parts []int
	for i := range list {
		sel, err := s.Select(list[i : i+1])
		if err != nil {
			t.Fatal(err)
		}
		for _, j := range sel {
			parts = append(parts, i+j)
		}
	}
	if fmt.Sprint(parts) != fmt.Sprint(whole) {
		t.Errorf("by parts %v, whole %v", parts, whole)
	}
	// A new Select of the second part alone decides by its first value,
	// a date, and fails on a later value that is not one.
	if _, err := f.Select(list[1:]); err != nil {
		t.Errorf("Select(list[1:]): %v", err)
	}
	var unsupported *resource.UnsupportedError
	v2, _ := resource.Decode([]byte(`[{"t":"2021-01-01T00:00:00Z"},{"t":"pending"}]`))
	if _, err := f.Select(v2.([]any)); !errors.As(err, &unsupported) {
		t.Errorf("a date, then text: %v", err)
	}
}
