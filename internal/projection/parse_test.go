package projection

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2/internal/resource"
)

func TestParse(t *testing.T) {
	for _, tt := range []struct {
		expr    string
		name    string
		columns string // key=label|align|sort|reverse|transforms, separated by ";"
	}{
		{"table", "table", ""},
		{" table ( name , zone ) ", "table", "name=NAME;zone=ZONE"},
		{"table[box,title='a, b'](name:label=N, zone.basename():align=right)", "table", "name=N;zone=ZONE|right|basename"},
		{"table(a.name, b.name, name, c.b.name)", "table", "a.name=NAME;b.name=B_NAME;name=NAME;c.b.name=C_B_NAME"},
		{"table(projectId, URLPath, getHTTPResponseCode, snake_case)", "table", "projectId=PROJECT_ID;URLPath=URL_PATH;getHTTPResponseCode=GET_HTTPRESPONSE_CODE;snake_case=SNAKE_CASE"},
		{"table(tags[0], tags[], tags[].len())", "table", "tags[0]=TAGS;tags[]=TAGS;tags[]=TAGS|len"},
		{"table(tags[].len(), tags[0])", "table", "tags[]=TAGS|len;tags[0]=TAGS|len"},
		{"table(name:sort=2:reverse, zone:sort=1, name:label=again)", "table", "name=NAME|sort2|reverse;zone=ZONE|sort1;name=again|sort2|reverse"},
		{"table(name.join(sep=' ', undefined=x).len())", "table", "name=NAME|join|len"},
		{"value(name)", "value", "name=NAME"},
		{"csv[no-heading,delimiter='|'](name)", "csv", "name=NAME"},
		{"json()", "json", ""},
		{"json(a.b[1])", "json", "a.b[1]=B"},
		{"flattened[no-pad]", "flattened", ""},
		{"list[compact,title=T]", "list", ""},
		{"none", "none", ""},
	} {
		f, err := Parse(tt.expr)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.expr, err)
			continue
		}
		if f.Name != tt.name {
			t.Errorf("Parse(%q).Name = %q, want %q", tt.expr, f.Name, tt.name)
		}
		var cols []string
		for _, c := range f.Columns {
			s := c.Key.String() + "=" + c.Label
			if c.Align != "left" {
				s += "|" + c.Align
			}
			if c.Sort > 0 {
				s += "|sort" + string(rune('0'+c.Sort))
			}
			if c.Reverse {
				s += "|reverse"
			}
			for _, tr := range c.Transforms {
				s += "|" + tr.name
			}
			cols = append(cols, s)
		}
		if got := strings.Join(cols, ";"); got != tt.columns {
			t.Errorf("Parse(%q) columns = %s, want %s", tt.expr, got, tt.columns)
		}
	}
}

func TestParseErrors(t *testing.T) {
	var syntax *resource.SyntaxError
	var unsupported *resource.UnsupportedError
	for _, tt := range []struct {
		expr        string
		unsupported bool
		contains    string
	}{
		{"", false, "Name expected"},
		{"bogus(name)", false, `unknown format "bogus"`},
		{"TABLE(name)", false, "unknown format"},
		{"table(name", false, "More tokens expected"},
		{"table(name zone)", false, "Expected )"},
		{"table[box", false, "More tokens expected"},
		{"table(name,)", false, "Non-empty key name expected"},
		{"table(name:align=middle)", false, "Unknown alignment"},
		{"table(name:label)", false, "value expected"},
		{"table(name:reverse=1)", false, "value not expected"},
		{"table(name) x", false, "unexpected text"},
		{"text", true, "text is clix's own format"},
		{"default", true, "the default format"},
		{"multi(a:format=json)", true, "the multi format"},
		{"table(name)[box]", true, "after the projection"},
		{"[box]table(name)", true, "does not start with its name"},
		{"table[foo](name)", true, `the table attribute "foo"`},
		{"table[box=1](name)", true, "a value for the box attribute"},
		{"table[title](name)", true, "title attribute without a value"},
		{"table[title=5](name)", true, "quote it"},
		{"table[pad=-1](name)", true, "pad=-1"},
		{"csv[box](name)", true, `the csv attribute "box"`},
		{"json[box](name)", true, `the json attribute "box"`},
		{"value(name:align=right)", true, "align key attribute with the value format"},
		{"table(name:wrap)", true, `the key attribute "wrap"`},
		{"table(name:label=5)", true, "as a label"},
		{"table(name:sort=0)", true, "sort=0"},
		{"table(name.date())", true, "the transform date()"},
		{"table(name.len(1))", true, "len() with 1 arguments"},
		{"table(name.join(sep=a,sep=b))", true, "sep given twice"},
		{"table(name.basename(x, y))", true, "basename() with 2 arguments"},
		{"table(name, NAME)", true, "label of an earlier column"},
		{"table(*name)", true, "map shorthand"},
		{"table(name[x])", true, "key index [x]"},
		{"table(len())", true, "empty or only a transform"},
		{"none(name)", true, "a projection with the none format"},
	} {
		_, err := Parse(tt.expr)
		switch {
		case err == nil:
			t.Errorf("Parse(%q) succeeded", tt.expr)
			continue
		case tt.unsupported && !errors.As(err, &unsupported):
			t.Errorf("Parse(%q): %v (%T), want an *UnsupportedError", tt.expr, err, err)
		case !tt.unsupported && !errors.As(err, &syntax):
			t.Errorf("Parse(%q): %v (%T), want a *SyntaxError", tt.expr, err, err)
		}
		if !strings.Contains(err.Error(), tt.contains) {
			t.Errorf("Parse(%q): %v, want it to mention %q", tt.expr, err, tt.contains)
		}
	}
}

func TestWithColumns(t *testing.T) {
	f, err := Parse("table[box]")
	if err != nil {
		t.Fatal(err)
	}
	if !f.NeedsColumns() {
		t.Fatal("table without a projection needs columns")
	}
	g, err := f.WithColumns([]resource.Key{{{Name: "name"}}, {{Name: "owner"}, {Name: "name"}}, {{Name: "name"}}})
	if err != nil {
		t.Fatal(err)
	}
	if g.NeedsColumns() || len(g.Columns) != 2 || g.Columns[1].Label != "OWNER_NAME" {
		t.Fatalf("WithColumns = %+v", g.Columns)
	}
	data, _ := resource.Decode([]byte(`[{"name":"a","owner":{"name":"o"}}]`))
	var b bytes.Buffer
	if err := g.Print(&b, data, Options{UTF8: false}); err != nil {
		t.Fatal(err)
	}
	want := "+------+------------+\n| NAME | OWNER_NAME |\n+------+------------+\n| a    | o          |\n+------+------------+\n"
	if b.String() != want {
		t.Errorf("ASCII box:\n%s\nwant\n%s", b.String(), want)
	}
	if h, _ := Parse("table(name)"); h.NeedsColumns() {
		t.Error("table(name) has columns")
	}
	if j, _ := Parse("json"); j.NeedsColumns() {
		t.Error("json never needs columns")
	}
}

func TestPrintErrors(t *testing.T) {
	data, _ := resource.Decode([]byte(`[{"name":"a\u001b[31mred","ok":"tab\there","wide":"é"}]`))
	var unsupported *resource.UnsupportedError
	for _, expr := range []string{"table(name)", "value(name)", "csv(name)", "flattened(name)", "list(name)"} {
		f, err := Parse(expr)
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		if err := f.Print(&b, data, Options{UTF8: true}); !errors.As(err, &unsupported) || b.Len() != 0 {
			t.Errorf("%s with an escape sequence: %v, printed %q; want an *UnsupportedError and nothing printed", expr, err, b.String())
		}
	}
	f, _ := Parse("table(ok, wide)")
	var b bytes.Buffer
	if err := f.Print(&b, data, Options{UTF8: true}); err != nil {
		t.Errorf("tab and é: %v", err)
	}
	if err := f.Print(&b, data, Options{UTF8: false}); !errors.As(err, &unsupported) {
		t.Errorf("é in an ASCII table: %v, want an *UnsupportedError", err)
	}
	for _, expr := range []string{"table()", "csv()", "value()"} {
		f, _ := Parse(expr)
		if err := f.Print(&b, data, Options{}); !errors.Is(err, ErrNeedsProjection) {
			t.Errorf("%s: %v, want ErrNeedsProjection", expr, err)
		}
	}
}

func TestEmptyData(t *testing.T) {
	for _, doc := range []string{`[]`, `{}`, `null`, `""`, `0`} {
		data, _ := resource.Decode([]byte(doc))
		for _, expr := range []string{"table(name)", "csv(name)", "value(name)", "flattened", "list", "none"} {
			f, _ := Parse(expr)
			var b bytes.Buffer
			if err := f.Print(&b, data, Options{UTF8: true}); err != nil || b.Len() != 0 {
				t.Errorf("%s on %s: %q, %v; want nothing", expr, doc, b.String(), err)
			}
		}
		f, _ := Parse("json(name)")
		if v, ok := f.Project(data); ok || len(v.([]any)) != 0 {
			t.Errorf("json(name) on %s: %v, %v; want no records", doc, v, ok)
		}
	}
	f, _ := Parse("list[always-display-title,title=T](name)")
	var b bytes.Buffer
	if err := f.Print(&b, []any{}, Options{}); err != nil || b.String() != "T\n" {
		t.Errorf("always-display-title: %q, %v", b.String(), err)
	}
}

func TestProjectAndJSON(t *testing.T) {
	data, _ := resource.Decode([]byte(`[{"name":"a","lab":{"env":"p","x":null},"tags":["x","y"],"n":1.5e16},{"name":"b"}]`))
	f, _ := Parse("json(name, lab.env, tags[1], n, missing)")
	v, ok := f.Project(data)
	if !ok {
		t.Fatal("no records")
	}
	got, err := JSON(v)
	if err != nil {
		t.Fatal(err)
	}
	if want := `[{"lab":{"env":"p"},"n":1.5e+16,"name":"a","tags":[null,"y"]},{"name":"b"}]`; string(got) != want {
		t.Errorf("JSON = %s, want %s", got, want)
	}
	single, _ := resource.Decode([]byte(`{"name":"a","x":1}`))
	v, _ = f.Project(single)
	if got, _ := JSON(v); string(got) != `{"name":"a"}` {
		t.Errorf("single record = %s", got)
	}
	if _, err := JSON([]any{posInfinity()}); err == nil {
		t.Error("JSON(inf) succeeded")
	}
	if got, _ := JSON("q\"\\\n\x01 é"); string(got) != `"q\"\\\n\u0001`+" "+`é"` {
		t.Errorf("JSON string = %s", got)
	}
}

func posInfinity() float64 { return infinity.Float }

func TestFloatText(t *testing.T) {
	for _, tt := range []struct {
		f    float64
		want string
	}{
		{1.5, "1.5"}, {100, "100.0"}, {1e16, "1e+16"}, {1234567, "1234567.0"}, {123456.789, "123456.8"},
		{12345.6789, "12345.7"}, {0.0001, "0.0001"}, {1e-5, "1e-05"}, {0.000123456789, "0.000123457"},
		{999999.5, "999999.5"}, {1e9, "1e+09"}, {123456789, "123456789.0"}, {-2.5, "-2.5"}, {0.1 + 0.2, "0.3"},
	} {
		if got := floatText(tt.f); got != tt.want {
			t.Errorf("floatText(%v) = %q, want %q", tt.f, got, tt.want)
		}
	}
}

func TestTextHelpers(t *testing.T) {
	if displayWidth("a全角😀́") != 1+4+2 || displayWidth("ab\ncdef") != 4 {
		t.Error("displayWidth")
	}
	if pyCenter("ab", 5) != "  ab " || pyCenter("ab", 6) != "  ab  " || pyCenter("abc", 6) != " abc  " || pyCenter("a", 4) != " a  " {
		t.Errorf("pyCenter: %q %q %q %q", pyCenter("ab", 5), pyCenter("ab", 6), pyCenter("abc", 6), pyCenter("a", 4))
	}
	if l, r := nextLine("one two\nthree", 7); l != "one two" || r != "three" {
		t.Errorf("nextLine = %q, %q", l, r)
	}
	if keyName(resource.Key{{Name: "a b"}, {Name: "c"}}) != `"a b".c` {
		t.Error("keyName quotes names")
	}
}
