package projection

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/SCKelemen/clix/v2/internal/resource"
)

// FuzzParse checks that Parse never panics, fails only with a
// *resource.SyntaxError or *resource.UnsupportedError, parses the same
// expression the same way twice, and that every format it accepts prints
// the fixture without panicking, failing only as documented.
func FuzzParse(f *testing.F) {
	for _, c := range goldenCases {
		f.Add(c.format)
	}
	for _, s := range []string{
		"", "table", "table()", "table(", "table[", "table[box", "json(a.b[0].c, d[])", "value(a.join(','), b.len())",
		"csv[separator=\"\\t\",terminator=';'](a)", "table(a:label='x:y':align=right:sort=1:reverse)", "table(a.basename(undefined=\"(\"))",
		"table(a, a, a[], a[0])", "table((a))", "table(a.b.c.d.e.f.g)", "flattened[no-pad,separator=' '](a)", "list[always-display-title,title='']",
		"table[title=\"\\\"\"](a)", "table(\"a\\\"b\")", "table('')", "table(a:label=)", "table(a:)", "table(a.)", "table(.)", "table([0])",
	} {
		f.Add(s)
	}
	data := goldenData(f)
	f.Fuzz(func(t *testing.T, expr string) {
		g, err := Parse(expr)
		if err != nil {
			var syntax *resource.SyntaxError
			var unsupported *resource.UnsupportedError
			if !errors.As(err, &syntax) && !errors.As(err, &unsupported) {
				t.Fatalf("Parse(%q): unexpected error type %T: %v", expr, err, err)
			}
			return
		}
		again, err := Parse(expr)
		if err != nil || !reflect.DeepEqual(g.Columns, again.Columns) || g.Name != again.Name {
			t.Fatalf("Parse(%q) twice differs: %v", expr, err)
		}
		for _, c := range g.Columns {
			if len(c.Key) == 0 {
				t.Fatalf("Parse(%q): a column with an empty key", expr)
			}
		}
		switch g.Name {
		case "json", "yaml":
			if v, ok := g.Project(data); ok {
				if _, err := JSON(v); err != nil {
					t.Fatalf("JSON of %q: %v", expr, err)
				}
			}
			return
		}
		if g.NeedsColumns() {
			if g, err = g.WithColumns([]resource.Key{{{Name: "name"}}, {{Name: "tags"}, {Name: "items"}}}); err != nil {
				t.Fatalf("WithColumns: %v", err)
			}
		}
		var b bytes.Buffer
		if err := g.Print(&b, data, Options{UTF8: true}); err != nil {
			var unsupported *resource.UnsupportedError
			if !errors.As(err, &unsupported) && !errors.Is(err, ErrNeedsProjection) {
				t.Fatalf("Print(%q): %v", expr, err)
			}
		}
	})
}
