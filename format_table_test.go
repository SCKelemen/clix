package clix

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2/internal/tty"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden files in testdata")

type tableOwner struct {
	Login string `json:"login"`
	ID    int    `json:"id"`
}

type tableLabel struct {
	Name string `json:"name"`
}

type tableRepo struct {
	Name        string        `json:"name"`
	DisplayName string        `json:"displayName"`
	ID          uint64        `json:"id"`
	Private     bool          `json:"private"`
	Stars       float64       `json:"stars"`
	Topics      []string      `json:"topics"`
	Owner       *tableOwner   `json:"owner"`
	Labels      []tableLabel  `json:"labels,omitempty"`
	Timeout     time.Duration `json:"timeout"`
	Created     time.Time     `json:"createdAt"`
	Secret      string        `json:"-"`
	Note        string        `json:"note,omitempty"`
}

var tableFixtures = map[string]struct {
	data    interface{}
	columns []string
}{
	"structs": {data: []tableRepo{
		{Name: "clix", DisplayName: "CLI toolkit", ID: 18446744073709551615, Stars: 4.5, Topics: []string{"cli", "go"},
			Owner: &tableOwner{Login: "sam", ID: 1}, Timeout: 90 * time.Second,
			Created: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Secret: "never printed"},
		{Name: "jq", DisplayName: "Query", ID: 2, Private: true, Labels: []tableLabel{{"bug"}, {"docs"}},
			Created: time.Date(2025, 12, 31, 23, 59, 59, 500000000, time.UTC), Note: "has a note"},
	}},
	"maps": {data: []map[string]interface{}{
		{"zeta": 1, "alpha": "a"},
		{"beta": true, "alpha": "b", "nested": map[string]interface{}{"k": []int{1, 2}}},
		{"alpha": nil},
	}},
	"columns": {
		data: []tableRepo{
			{Name: "clix", Owner: &tableOwner{Login: "sam"}, Labels: []tableLabel{{"bug"}, {"ui"}}},
			{Name: "jq"},
		},
		columns: []string{"owner.login", "name", "labels.name", "missing"},
	},
	"escapes": {data: []map[string]string{
		{"name": "tab\there", "path": `C:\dir`},
		{"name": "two\nlines", "path": "esc\x1b[31mred\x1b[0m"},
		{"name": "cr\rx", "path": "del\x7f c1\u0085"},
	}},
	"wide": {data: []map[string]string{
		{"name": "日本語", "emoji": "👍🏽 ok", "plain": "x"},
		{"name": "abc", "emoji": "🇸🇪", "plain": "longer value"},
	}},
	"long": {data: []map[string]string{
		{"id": "1", "title": "A fairly long title that will not fit in a narrow terminal", "state": "open", "url": "https://example.com/issues/1"},
		{"id": "22", "title": "Short", "state": "closed", "url": "https://example.com/issues/22"},
	}},
}

var tableModes = map[string]textTerminal{
	"tsv":    {},
	"tty":    {on: true, width: 200, ellipsis: "…"},
	"color":  {on: true, width: 200, color: true, ellipsis: "…"},
	"narrow": {on: true, width: 40, ellipsis: "..."},
}

// Lists of objects print as aligned tables on a terminal and as
// tab-separated values elsewhere. The golden files in testdata/table hold
// each fixture in each mode; run "go test -run TestTableGolden -update" to
// rewrite them.
func TestTableGolden(t *testing.T) {
	for name, fx := range tableFixtures {
		for mode, term := range tableModes {
			t.Run(name+"/"+mode, func(t *testing.T) {
				node := textNode(fx.data)
				rows, ok := tableRows(node)
				if !ok {
					t.Fatalf("fixture %s is not a list of objects: %#v", name, node)
				}
				var b bytes.Buffer
				if err := writeTable(&b, fx.data, rows, FormatOptions{Columns: fx.columns}, term); err != nil {
					t.Fatal(err)
				}
				checkGolden(t, filepath.Join("testdata", "table", name+"."+mode+".golden"), b.String())

				switch mode {
				case "tsv":
					// Escape sequences in the data pass through; TSV adds none.
					if name != "escapes" && strings.ContainsRune(b.String(), 0x1b) {
						t.Errorf("TSV output has an escape sequence: %q", b.String())
					}
					for _, line := range strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n") {
						if got, want := strings.Count(line, "\t"), columnCount(fx.data, fx.columns)-1; got != want {
							t.Errorf("TSV line %q has %d tabs, want %d", line, got, want)
						}
					}
				case "tty", "narrow":
					if strings.ContainsRune(b.String(), 0x1b) {
						t.Errorf("table without color has an escape sequence: %q", b.String())
					}
					for _, line := range strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n") {
						if w := tty.StringWidth(line); w > term.width {
							t.Errorf("line %q is %d columns wide, more than %d", line, w, term.width)
						}
						if strings.HasSuffix(line, " ") {
							t.Errorf("line %q ends with a space", line)
						}
					}
				}
			})
		}
	}
}

func columnCount(data interface{}, columns []string) int {
	if len(columns) > 0 {
		return len(columns)
	}
	rows, _ := tableRows(textNode(data))
	return len(tableColumns(data, rows))
}

func checkGolden(t *testing.T, path, got string) {
	t.Helper()
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if got != string(want) {
		t.Errorf("output differs from %s:\ngot:\n%s\nwant:\n%s", path, got, want)
	}
}

// Columns come in the order of the JSON output: declaration order for
// structs, with an omitempty field placed where it is declared, and
// sorted keys for maps.
func TestTableColumnOrder(t *testing.T) {
	type row struct {
		First  string `json:"first,omitempty"`
		Second string `json:"second"`
		Third  string `json:"third,omitempty"`
		Fourth string `json:"fourth"`
	}
	for _, tc := range []struct {
		name string
		data interface{}
		want []string
	}{
		{"declared", []row{{Second: "b"}, {First: "a", Second: "b", Third: "c"}}, []string{"first", "second", "third", "fourth"}},
		{"later omitempty", []row{{First: "a"}, {Third: "c"}}, []string{"first", "second", "third", "fourth"}},
		{"maps", []map[string]int{{"b": 1}, {"a": 2}, {"c": 3, "a": 1}}, []string{"a", "b", "c"}},
		{"maps disjoint", []map[string]int{{"a": 1}, {"b": 2}}, []string{"a", "b"}},
	} {
		rows, ok := tableRows(textNode(tc.data))
		if !ok {
			t.Fatalf("%s: not a table", tc.name)
		}
		if got := tableColumns(tc.data, rows); strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("%s: columns %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestColumnHeader(t *testing.T) {
	for in, want := range map[string]string{
		"name":         "NAME",
		"displayName":  "DISPLAY NAME",
		"display_name": "DISPLAY NAME",
		"display-name": "DISPLAY NAME",
		"owner.login":  "OWNER LOGIN",
		"userID":       "USER ID",
		"URLPath":      "URL PATH",
		"ID":           "ID",
		"item2Name":    "ITEM2 NAME",
		"createdAt":    "CREATED AT",
	} {
		if got := columnHeader(in); got != want {
			t.Errorf("columnHeader(%q) = %q, want %q", in, got, want)
		}
	}
}

// Only lists whose elements are all objects are tables; everything else
// prints as before.
func TestTableOnlyForListsOfObjects(t *testing.T) {
	for _, tc := range []struct {
		name  string
		data  interface{}
		table bool
	}{
		{"structs", []tableOwner{{Login: "a"}}, true},
		{"maps", []map[string]int{{"a": 1}}, true},
		{"pointers", []*tableOwner{{Login: "a"}}, true},
		{"nil element", []*tableOwner{{Login: "a"}, nil}, false},
		{"mixed", []interface{}{map[string]int{"a": 1}, 2}, false},
		{"scalars", []int{1, 2}, false},
		{"empty", []tableOwner{}, false},
		{"lists", [][]int{{1}, {2}}, false},
		{"object", tableOwner{Login: "a"}, false},
	} {
		if _, got := tableRows(textNode(tc.data)); got != tc.table {
			t.Errorf("%s: table = %v, want %v", tc.name, got, tc.table)
		}
	}
}

// FormatDataWith passes Columns and HeaderStyle to the table.
func TestFormatDataWithColumns(t *testing.T) {
	data := []tableOwner{{Login: "a", ID: 1}, {Login: "b", ID: 2}}
	var b bytes.Buffer
	if err := FormatDataWith(&b, data, FormatOptions{Format: "TEXT", Columns: []string{"id", "login"}}); err != nil {
		t.Fatal(err)
	}
	if got, want := b.String(), "1\ta\n2\tb\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	var styled bytes.Buffer
	term := textTerminal{on: true, width: 80, color: true}
	rows, _ := tableRows(textNode(data))
	style := StyleFunc(func(s ...string) string { return "<" + strings.Join(s, "") + ">" })
	if err := writeTable(&styled, data, rows, FormatOptions{HeaderStyle: style}, term); err != nil {
		t.Fatal(err)
	}
	if got, want := styled.String(), "<LOGIN>  <ID>\na      1\nb      2\n"; got != want {
		t.Errorf("styled header: got %q, want %q", got, want)
	}

	// JSON and YAML are unaffected by Columns.
	var j bytes.Buffer
	if err := FormatDataWith(&j, data, FormatOptions{Format: FormatJSON, Columns: []string{"id"}}); err != nil {
		t.Fatal(err)
	}
	var plain bytes.Buffer
	_ = FormatData(&plain, data, FormatJSON)
	if j.String() != plain.String() {
		t.Errorf("Columns changed JSON output: %q vs %q", j.String(), plain.String())
	}
}
