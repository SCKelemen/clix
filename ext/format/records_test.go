package format_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
)

// describe writes a filter tree as kind(operands...), with terms as
// key op operands.
func describe(e format.FilterExpr) string {
	join := func(es []format.FilterExpr) string {
		var parts []string
		for _, e := range es {
			parts = append(parts, describe(e))
		}
		return strings.Join(parts, ", ")
	}
	switch e := e.(type) {
	case nil:
		return "nil"
	case *format.FilterTerm:
		list := ""
		if e.IsList() {
			list = " list"
		}
		return fmt.Sprintf("%s %s %q%s", e.Key(), e.Op(), e.Operands(), list)
	case *format.FilterAnd:
		return "and(" + join(e.Operands()) + ")"
	case *format.FilterOr:
		return "or(" + join(e.Operands()) + ")"
	case *format.FilterNot:
		return "not(" + describe(e.Operand()) + ")"
	}
	return fmt.Sprintf("unknown %T", e)
}

func mustFilter(t *testing.T, expr string) *format.Filter {
	t.Helper()
	f, err := format.ParseFilter(expr)
	if err != nil {
		t.Fatalf("ParseFilter(%q): %v", expr, err)
	}
	return f
}

func TestFilterExpr(t *testing.T) {
	for _, tt := range []struct{ expr, tree, str string }{
		{"", "nil", ""},
		{"name:web*", `name : ["web*"]`, "name:web*"},
		{
			"name:web* AND (status=RUNNING OR -labels.env:*) tags.items=(web db)",
			`and(name : ["web*"], or(status = ["RUNNING"], not(labels.env : ["*"])), tags.items = ["web" "db"] list)`,
			"name:web* AND (status=RUNNING OR NOT labels.env:*) AND tags.items=(web,db)",
		},
		{"a=1 b=2 OR c=3", `and(a = ["1"], or(b = ["2"], c = ["3"]))`, "a=1 AND (b=2 OR c=3)"},
		{"(a<1 AND b>=2) OR NOT (c!=3 OR d<=4)", `or(and(a < ["1"], b >= ["2"]), not(or(c != ["3"], d <= ["4"])))`,
			"(a<1 AND b>=2) OR NOT (c!=3 OR d<=4)"},
		{`"k.e.y".value:'a b'`, `"k.e.y".value : ["a b"]`, `"k.e.y".value:"a b"`},
	} {
		f := mustFilter(t, tt.expr)
		if got := describe(f.Expr()); got != tt.tree {
			t.Errorf("ParseFilter(%q).Expr() is\n %s\nwant\n %s", tt.expr, got, tt.tree)
		}
		if got := f.String(); got != tt.str {
			t.Errorf("String() = %q, want %q", got, tt.str)
		}
		if e := f.Expr(); e != nil && e.String() != tt.str {
			t.Errorf("Expr().String() = %q, want %q", e.String(), tt.str)
		}
		// The canonical form parses into the same tree.
		if got := describe(mustFilter(t, f.String()).Expr()); got != tt.tree {
			t.Errorf("ParseFilter(%q) is %s, want %s", f.String(), got, tt.tree)
		}
	}
}

func TestFilterExprAccessors(t *testing.T) {
	f := mustFilter(t, "disks[0].name=boot tags.items[]:web labels.env=(prod) x>=-1")
	and, ok := f.Expr().(*format.FilterAnd)
	if !ok {
		t.Fatalf("Expr() = %T", f.Expr())
	}
	if f.Expr() != and || and.Operands()[0] != f.Expr().(*format.FilterAnd).Operands()[0] {
		t.Error("Expr returns different nodes on each call")
	}
	ops := and.Operands()
	ops[0] = nil // a copy
	if and.Operands()[0] == nil {
		t.Error("Operands returned the node's own slice")
	}
	terms := make([]*format.FilterTerm, 0, 4)
	for _, e := range and.Operands() {
		terms = append(terms, e.(*format.FilterTerm))
	}
	parts := terms[0].Key().Parts()
	if len(parts) != 3 || parts[0].Name != "disks" || parts[1].Index == nil || parts[1].Index.Int64() != 0 || parts[2].Name != "name" {
		t.Errorf("disks[0].name parts %+v", parts)
	}
	parts[1].Index.SetInt64(5) // a copy
	if terms[0].Key().String() != "disks[0].name" {
		t.Errorf("Key().String() = %q after changing a copy", terms[0].Key())
	}
	if parts := terms[1].Key().Parts(); len(parts) != 3 || !parts[2].Slice || parts[2].Name != "" || parts[2].Index != nil {
		t.Errorf("tags.items[] parts %+v", parts)
	}
	if terms[2].Op() != format.FilterEqual || !terms[2].IsList() || !slices.Equal(terms[2].Operands(), []string{"prod"}) {
		t.Errorf("labels.env=(prod): %s %v %q", terms[2].Op(), terms[2].IsList(), terms[2].Operands())
	}
	if terms[3].Op() != format.FilterGreaterEqual || terms[3].IsList() || terms[3].String() != "x>=-1" {
		t.Errorf("x>=-1: %s %v %s", terms[3].Op(), terms[3].IsList(), terms[3])
	}
	ops2 := map[format.FilterOp]string{format.FilterHas: ":", format.FilterEqual: "=", format.FilterNotEqual: "!=",
		format.FilterLess: "<", format.FilterLessEqual: "<=", format.FilterGreater: ">", format.FilterGreaterEqual: ">="}
	for op, text := range ops2 {
		e := mustFilter(t, "a"+text+"1").Expr().(*format.FilterTerm)
		if e.Op() != op {
			t.Errorf("a%s1: Op() = %q", text, e.Op())
		}
	}
}

// The nil *Filter, the zero Filter and an empty expression select every
// record, and so do their selectors and the zero and nil FilterSelector.
func TestFilterEmpty(t *testing.T) {
	var nilFilter *format.Filter
	for _, f := range []*format.Filter{nilFilter, new(format.Filter), mustFilter(t, ""), mustFilter(t, "()"), format.NewFilter(nil)} {
		if f.Expr() != nil || f.String() != "" {
			t.Errorf("%v: Expr %v, String %q", f, f.Expr(), f.String())
		}
		got, err := f.Select(projects)
		if err != nil || fmt.Sprint(got) != "[0 1 2 3]" {
			t.Errorf("Select = %v, %v", got, err)
		}
		if got, err := f.Selector().Select(projects); err != nil || fmt.Sprint(got) != "[0 1 2 3]" {
			t.Errorf("Selector().Select = %v, %v", got, err)
		}
		if got, err := (format.ListOptions{Filter: f}).Select(projects); err != nil || fmt.Sprint(got) != "[0 1 2 3]" {
			t.Errorf("ListOptions.Select = %v, %v", got, err)
		}
		server, client := f.Split(func(format.FilterExpr) bool { t.Error("called without conjuncts"); return true })
		if server.Expr() != nil || client.Expr() != nil || f.Conjuncts() != nil {
			t.Errorf("Split = %q, %q; Conjuncts %v", server, client, f.Conjuncts())
		}
	}
	var nilSelector *format.FilterSelector
	for _, s := range []*format.FilterSelector{nilSelector, new(format.FilterSelector)} {
		for range 2 {
			if got, err := s.Select(projects); err != nil || fmt.Sprint(got) != "[0 1 2 3]" {
				t.Errorf("%#v: Select = %v, %v", s, got, err)
			}
		}
	}
}

// NewFilter panics on a node no parse produced, rather than selecting
// every record or failing later.
func TestNewFilterForeignNode(t *testing.T) {
	var nilTerm *format.FilterTerm
	for _, e := range []format.FilterExpr{&format.FilterTerm{}, &format.FilterAnd{}, &format.FilterOr{}, &format.FilterNot{}, nilTerm} {
		func() {
			defer func() {
				r := recover()
				if msg, _ := r.(string); !strings.Contains(msg, "Filter.Expr did not return") {
					t.Errorf("NewFilter(%#v) panicked with %v", e, r)
				}
			}()
			format.NewFilter(e)
			t.Errorf("NewFilter(%#v) did not panic", e)
		}()
	}
}

func TestFilterSplit(t *testing.T) {
	// keys sends the terms on these keys to the server, and an OR or NOT
	// when all of its terms are.
	keys := func(names ...string) func(format.FilterExpr) bool {
		var ok func(format.FilterExpr) bool
		ok = func(e format.FilterExpr) bool {
			switch e := e.(type) {
			case *format.FilterTerm:
				return slices.Contains(names, e.Key().String())
			case *format.FilterOr:
				return !slices.ContainsFunc(e.Operands(), func(e format.FilterExpr) bool { return !ok(e) })
			case *format.FilterNot:
				return ok(e.Operand())
			}
			return false
		}
		return ok
	}
	for _, tt := range []struct {
		expr           string
		server         func(format.FilterExpr) bool
		onServer, rest string
	}{
		{"a=1 b=2 c=3", keys("a", "c"), "a=1 AND c=3", "b=2"},
		{"a=1 AND (b=2 AND c=3)", keys("c"), "c=3", "a=1 AND b=2"},
		{"a=1 b=2", keys("a", "b"), "a=1 AND b=2", ""},
		{"a=1 b=2", keys(), "", "a=1 AND b=2"},
		{"a=1 b=2", nil, "", "a=1 AND b=2"},
		{"a=1", keys("a"), "a=1", ""},
		{"a=1", keys("b"), "", "a=1"},
		// An OR is one conjunct: it goes to the server whole or not at all.
		{"a=1 OR b=2", keys("a"), "", "a=1 OR b=2"},
		{"a=1 OR b=2", keys("a", "b"), "a=1 OR b=2", ""},
		{"c=3 (a=1 OR b=2)", keys("a", "c"), "c=3", "a=1 OR b=2"},
		// So is a NOT.
		{"NOT (a=1 AND b=2) c=3", keys("a", "c"), "c=3", "NOT (a=1 AND b=2)"},
		{"-a=1 b=2", keys("a"), "NOT a=1", "b=2"},
	} {
		f := mustFilter(t, tt.expr)
		var seen []string
		pred := tt.server
		if pred != nil {
			pred = func(e format.FilterExpr) bool {
				seen = append(seen, e.String())
				return tt.server(e)
			}
		}
		server, client := f.Split(pred)
		if server.String() != tt.onServer || client.String() != tt.rest {
			t.Errorf("%q: Split = %q, %q; want %q, %q", tt.expr, server, client, tt.onServer, tt.rest)
		}
		// The conjuncts, each once, in order.
		var want []string
		switch e := f.Expr().(type) {
		case *format.FilterAnd:
			for _, c := range e.Operands() {
				want = append(want, c.String())
			}
		default:
			want = []string{e.String()}
		}
		if pred != nil && !slices.Equal(seen, want) {
			t.Errorf("%q: the predicate saw %q, want %q", tt.expr, seen, want)
		}
		var conjuncts []string
		for _, c := range f.Conjuncts() {
			conjuncts = append(conjuncts, c.String())
		}
		if !slices.Equal(conjuncts, want) {
			t.Errorf("%q: Conjuncts() = %q, want %q", tt.expr, conjuncts, want)
		}
		// Both parts are subtrees of f: the same nodes.
		if e, ok := server.Expr().(*format.FilterAnd); ok {
			for _, c := range e.Operands() {
				if !slices.ContainsFunc(f.Expr().(*format.FilterAnd).Operands(), func(d format.FilterExpr) bool { return d == c }) {
					t.Errorf("%q: %s is not a node of f", tt.expr, c)
				}
			}
		}
	}
}

// instances are records with a mix of values for the split tests.
var instances = []map[string]any{
	{"name": "web-1", "status": "RUNNING", "labels": map[string]any{"env": "prod"}, "cpus": 2, "tags": []string{"web", "http"}},
	{"name": "web-2", "status": "TERMINATED", "labels": map[string]any{"env": "prod"}, "cpus": 4, "tags": []string{"web"}},
	{"name": "db-1", "status": "running", "labels": map[string]any{"env": "test"}, "cpus": 16},
	{"name": "Batch", "status": "RUNNING", "labels": map[string]any{}, "cpus": 8, "tags": []string{}},
	{"name": "web-3", "status": "STOPPING", "cpus": 1, "tags": []string{"http"}},
}

// pick returns the records of list at the indices.
func pick[T any](list []T, indices []int) []T {
	out := make([]T, len(indices))
	for i, j := range indices {
		out[i] = list[j]
	}
	return out
}

// A server that evaluates exactly what Split gives it, followed by the
// client part on its results, selects what f does; one that returns more
// needs f itself on the client. The client part never adds a record.
func TestFilterSplitResidue(t *testing.T) {
	exprs := []string{
		"status=RUNNING name:web*",
		"status=RUNNING AND (labels.env=prod OR cpus>4) -name:batch",
		"labels.env:* cpus>=2 tags:web",
		"NOT status=running cpus<10",
		"(name:web* AND cpus>1) OR name=db-1",
		"name:web* name:*1",
	}
	for _, expr := range exprs {
		f := mustFilter(t, expr)
		want, err := f.Select(instances)
		if err != nil {
			t.Fatalf("%q: %v", expr, err)
		}
		n := 1
		if and, ok := f.Expr().(*format.FilterAnd); ok {
			n = len(and.Operands())
		}
		// Every way to divide the conjuncts between server and client.
		for mask := range 1 << n {
			i := -1
			server, client := f.Split(func(format.FilterExpr) bool { i++; return mask&(1<<i) != 0 })

			onServer, err := server.Select(instances)
			if err != nil {
				t.Fatal(err)
			}
			returned := pick(instances, onServer)
			kept, err := client.Select(returned)
			if err != nil {
				t.Fatal(err)
			}
			if got := pick(onServer, kept); !slices.Equal(got, want) {
				t.Errorf("%q split %q + %q selects %v, want %v", expr, server, client, got, want)
			}

			// A server that ignores its filter and returns everything: the
			// client part alone keeps a superset, and f itself is exact.
			broad, err := client.Select(instances)
			if err != nil {
				t.Fatal(err)
			}
			for _, j := range want {
				if !slices.Contains(broad, j) {
					t.Errorf("%q: the client part %q drops %d", expr, client, j)
				}
			}
			exact, err := f.Select(instances)
			if err != nil || !slices.Equal(exact, want) {
				t.Errorf("%q on everything: %v, %v", expr, exact, err)
			}

			// NewFilter evaluates a subtree as if it were the whole filter.
			for _, part := range []*format.Filter{server, client} {
				if e := part.Expr(); e != nil {
					a, errA := format.NewFilter(e).Select(instances)
					b, errB := mustFilter(t, e.String()).Select(instances)
					if fmt.Sprint(a, errA) != fmt.Sprint(b, errB) {
						t.Errorf("NewFilter(%s) selects %v, %v; parsed, %v, %v", e, a, errA, b, errB)
					}
				}
			}
		}
	}
}

// A FilterSelector given pages selects what Select of all of them does,
// including the decision a term makes on its first value.
func TestFilterSelector(t *testing.T) {
	records := []map[string]any{{"t": "pending"}, {"t": "2016-01-01T00:00:00Z"}, {"t": "2021-01-01T00:00:00Z"}, {"t": "abc"}}
	f := mustFilter(t, "t<2018-01-01")
	want, err := f.Select(records)
	if err != nil {
		t.Fatal(err)
	}
	s := f.Selector()
	var got []int
	for start := 0; start < len(records); start += 2 {
		sel, err := s.Select(records[start : start+2])
		if err != nil {
			t.Fatal(err)
		}
		for _, j := range sel {
			got = append(got, start+j)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("by pages %v, whole %v", got, want)
	}
	// Each Select alone decides on its own first value: the second page
	// starts with a date, so the later "abc" cannot be compared.
	if _, err := f.Select(records[1:]); err == nil || !strings.Contains(err.Error(), "is not supported in clix --filter") {
		t.Errorf("Select(records[1:]) = %v", err)
	}
}

func TestSortBy(t *testing.T) {
	s, err := format.ParseSortBy("labels.env,~name,tags[]")
	if err != nil {
		t.Fatal(err)
	}
	if s.String() != "labels.env,~name,tags[]" {
		t.Errorf("String() = %q", s)
	}
	var keys []string
	for _, k := range s.Keys() {
		keys = append(keys, fmt.Sprintf("%s %v", k.Key, k.Descending))
	}
	if got := strings.Join(keys, "; "); got != "labels.env false; name true; tags[0] false" {
		t.Errorf("Keys() = %s", got)
	}
	order, err := s.Sort(projects)
	if err != nil || fmt.Sprint(order) != "[2 0 3 1]" {
		t.Errorf("Sort = %v, %v", order, err)
	}
	if order, err := (format.SortBy{}).Sort(projects); err != nil || fmt.Sprint(order) != "[0 1 2 3]" {
		t.Errorf("the zero SortBy: %v, %v", order, err)
	}
	if _, err := format.ParseSortBy("a,,b"); err == nil || !strings.Contains(err.Error(), "empty field") {
		t.Errorf("ParseSortBy(a,,b) = %v", err)
	}
}

func TestRecordsShapes(t *testing.T) {
	f := mustFilter(t, "labels.env=prod")
	arr := [4]project(projects)
	for _, records := range []any{projects, arr, &projects, listedJSON(projects), []any{projects[0], projects[1], projects[2], projects[3]}} {
		got, err := f.Select(records)
		if err != nil || fmt.Sprint(got) != "[0 3]" {
			t.Errorf("%T: %v, %v", records, got, err)
		}
	}
	raw := make([]json.RawMessage, len(projects))
	for i, p := range projects {
		raw[i], _ = json.Marshal(p)
	}
	if got, err := format.ApplyList(raw, format.ListOptions{Filter: f}); err != nil || len(got) != 2 || !bytes.Equal(got[1], raw[3]) {
		t.Errorf("ApplyList on JSON: %s, %v", got, err)
	}
	if got, err := f.Select(nil); err != nil || len(got) != 0 {
		t.Errorf("Select(nil) = %v, %v", got, err)
	}
	if _, err := f.Select(projects[0]); err == nil || err.Error() != "format: the records are a single object, not a list" {
		t.Errorf("Select(one object) = %v", err)
	}
	if _, err := (format.SortBy{}).Sort("text"); err == nil || !strings.Contains(err.Error(), "a single string") {
		t.Errorf("Sort(a string) = %v", err)
	}
}

// ListOptions select exactly the records Write prints with the same list
// flags, and fail where Write does, with its messages.
func TestListOptionsMatchWrite(t *testing.T) {
	app, out, ran := listApp(t, true)
	filters := []string{"", "lifecycleState=ACTIVE", "labels.env=prod", "name:cafe", "projectNumber>1000 OR project_id=old",
		"createTime>=2018-01-15T00:00:00Z", "-labels.env:*", "name:web* AND NOT labels.env=test", "name:nope"}
	sorts := []string{"", "projectId", "~projectId", "labels.env,~name", "projectNumber", "~createTime"}
	limits := []int{0, 1, 2, 10}
	for _, fe := range filters {
		for _, se := range sorts {
			for _, limit := range limits {
				args := []string{"projects", "list", "--format", "json"}
				var o format.ListOptions
				if fe != "" {
					args = append(args, "--filter", fe)
					o.Filter = mustFilter(t, fe)
				}
				if se != "" {
					args = append(args, "--sort-by", se)
					var err error
					if o.SortBy, err = format.ParseSortBy(se); err != nil {
						t.Fatal(err)
					}
				}
				if limit > 0 {
					args = append(args, "--limit", fmt.Sprint(limit))
					o.Limit = limit
				}
				printed, err := runList(t, app, out, ran, args...)
				if err != nil {
					t.Fatalf("%q: %v", args, err)
				}
				kept, err := format.ApplyList(projects, o)
				if err != nil {
					t.Fatalf("%q: ApplyList: %v", args, err)
				}
				var b bytes.Buffer
				if err := clix.FormatData(&b, kept, clix.FormatJSON); err != nil {
					t.Fatal(err)
				}
				if b.String() != printed {
					t.Errorf("%q: Write printed\n%s\nApplyList kept\n%s", args, printed, b.String())
				}
			}
		}
	}

	// Errors from the data are Write's, less the usage error's flag.
	for _, tc := range []struct{ flag, value string }{{"--filter", "createTime:2021"}, {"--sort-by", "projectNumber.__doc__"}} {
		_, writeErr := runList(t, app, out, ran, "projects", "list", tc.flag, tc.value)
		var o format.ListOptions
		var err error
		if tc.flag == "--filter" {
			o.Filter = mustFilter(t, tc.value)
		} else if o.SortBy, err = format.ParseSortBy(tc.value); err != nil {
			t.Fatal(err)
		}
		_, opsErr := o.Select(projects)
		if writeErr == nil || opsErr == nil || writeErr.Error() != tc.flag+": "+opsErr.Error() {
			t.Errorf("%s %s: Write %v, Select %v", tc.flag, tc.value, writeErr, opsErr)
		}
	}
}

// ListOptions apply filter, then sort, then limit.
func TestListOptionsOrder(t *testing.T) {
	sortBy, err := format.ParseSortBy("~createTime")
	if err != nil {
		t.Fatal(err)
	}
	o := format.ListOptions{Filter: mustFilter(t, "labels.env=prod"), SortBy: sortBy, Limit: 1}
	got, err := o.Select(projects)
	if err != nil || fmt.Sprint(got) != "[0]" {
		t.Errorf("Select = %v, %v", got, err)
	}
	o.Limit = -1 // keeps all
	if got, err := format.ApplyList(projects, o); err != nil || len(got) != 2 || got[0].ProjectID != "web-prod" || got[1].ProjectID != "api" {
		t.Errorf("ApplyList = %v, %v", got, err)
	}
	if got, err := format.ApplyList([]project(nil), o); err != nil || got == nil || len(got) != 0 {
		t.Errorf("ApplyList(nil) = %#v, %v", got, err)
	}
}
