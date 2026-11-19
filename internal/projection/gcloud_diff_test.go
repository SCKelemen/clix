package projection

import (
	"bytes"
	"errors"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2/internal/gcloudtest"
	"github.com/SCKelemen/clix/v2/internal/resource"
)

// printScript prints each case with gcloud's resource_printer. For json
// formats it prints the canonical JSON of what the printer printed, so the
// test compares the projected data rather than the encoding.
const printScript = `
import io
from googlecloudsdk.core.resource import resource_printer
out = []
for c in json.load(sys.stdin):
    buf = io.StringIO()
    try:
        resource_printer.Print(json.loads(c["json"]), c["format"], out=buf)
        text = buf.getvalue()
        if c["format"].startswith("json"):
            text = json.dumps(json.loads(text), sort_keys=True, ensure_ascii=False)
        out.append({"out": text})
    except Exception as e:
        out.append({"error": type(e).__name__ + ": " + str(e)})
print(json.dumps(out))
`

type printCase struct {
	Format string `json:"format"`
	JSON   string `json:"json"`
}

type printResult struct {
	Out   string `json:"out"`
	Error string `json:"error"`
}

var diffDocs = []string{
	`[{"name":"vm-1","zone":"us-central1-a","n":10,"f":1.5,"b":true,"tags":["x","y"],"lab":{"env":"prod","k":1},"z":null,"selfLink":"https://x/projects/p/zones/us-a/instances/vm-1"},
	  {"name":"a-much-longer-name","zone":"eu","n":0,"f":1e16,"b":false,"tags":[],"lab":{},"z":5,"selfLink":"c:\\dir\\file"},
	  {"name":"三文字","zone":"asia","n":-3,"f":0.0001,"b":null,"tags":["only"],"lab":{"a b":"c,d"},"selfLink":""}]`,
	`[{"desc":"line one\nline two\nthree","id":1,"v":" padded "},{"desc":"single","id":22,"v":"q\"uote"},{"desc":"a\n\nb","id":333,"v":"semi;colon,comma"}]`,
	`[{"a":{"b":[{"c":1},{"c":2,"d":[null,3,null]}],"e":null},"x":[1,null,null]},{"a":{"b":[]},"x":[null]},{"a":null,"x":[[1,2],[]]}]`,
	`{"name":"single","items":[{"k":"v"}],"nested":{"deep":{"er":true}}}`,
	`[{"projectId":"p1","createTime":"2021-02-10T19:19:49.242Z","lifecycleState":"ACTIVE","projectNumber":"123","parent":{"id":"9","type":"folder"}},
	  {"projectId":"p2","createTime":"2020-01-01T00:00:00Z","lifecycleState":"DELETE_REQUESTED","projectNumber":"45","parent":{"id":"10","type":"organization"}}]`,
	`[]`,
	`[{"n":1.0},{"n":100.0},{"n":1234567.0},{"n":12345678901.0},{"n":0.5},{"n":-2.25e-7},{"n":3},{"n":true},{"n":"3"},{"n":null}]`,
	`["plain", "strings", "", "x"]`,
	`[1, 0, true, false, null, 2.5, [1,2], {"k":"v"}]`,
	`[{"projectId":"p","URLPath":"/a/b","getHTTPResponseCode":200,"a":{"name":"an"},"b":{"name":"bn"},"name":"top","snake_case":"s","k.e.y":"dots","with space":"sp"}]`,
	`[{"w":"全角文字です","e":"😀😀","c":"e\u0301e","s":"   ","t":"tab\there","u":"ünïcödé"},{"w":"x","e":"y","c":"z","s":"","t":"","u":"ascii"}]`,
	`[{"l":[0,false,"",null,[],{},"a"],"d":{"z":1,"a":null,"m":[1,2],"n":{"x":"y"}},"s":"a,b","q":" lead","r":"trail ","x":"\"q\""}]`,
	`[{"long":"word word word word word word word word word word word word\nsecond line here","id":1}]`,
	`{}`,
	`null`,
	`"a string resource"`,
}

var diffFormats = []string{
	"table(name,zone,n,f,b,tags,lab,z)",
	"table(name, zone.basename(), selfLink.basename():label=LINK, tags.join(','), tags.len(), lab.len())",
	"table[box](name,zone)", "table[all-box](name,zone,tags)", "table[box,title=Hello](name,zone)",
	"table[title='A much longer title than the columns'](name,n)", "table[box,title='A much longer title than the columns'](n)",
	"table[no-heading](name,zone)", "table[pad=4](name,zone)", "table[pad=0](name,zone)",
	"table(name:label=NAME_X, zone:label='')", "table(name:label='', zone:label='')", "table(name:align=right, n:align=center, zone)",
	"table(name:sort=1, zone)", "table(name, zone:reverse)", "table(b:sort=2, name:sort=1:reverse)", "table(n:sort=1, name)", "table(f:sort=1:reverse,name)",
	"table(desc,id,v)", "table[box](desc,id,v)", "table(id:sort=1:reverse,desc)",
	"table(a.b,a.b[0].c,a.b[1].d,x,x[0],a.e)", "table(a.b[].c, a)", "table(name, items[0].k, nested.deep.er)",
	"table(projectId, createTime, lifecycleState, parent.id, parent.type, projectNumber)",
	"table(parent.id, id, parent.type, type)",
	"table(n)", "table(n:sort=1)", "table(n:sort=1:reverse)",
	"csv(name,zone,n,f,b,tags,lab,z)", "csv[no-heading](name,tags)", "csv[separator=';'](name,tags)", "csv[delimiter='|',terminator='\\n--\\n'](name,tags,lab)",
	"csv[no-quote](desc,v)", "csv(desc,id,v)", "csv(name:label=NAME_LABEL,zone)", "csv(a.b,x)", "csv(n)",
	"value(name,tags,lab,f)", "value(desc,v)", "value[quote](desc,v)", "value[separator=','](name,zone)", "value[delimiter=' '](tags)", "value(n)",
	"value(a,x)", "value(name)", "value(items, nested)",
	"flattened", "flattened(name,tags)", "flattened(lab.env, name)", "flattened[no-pad](name,zone)", "flattened[separator=' = '](name)",
	"flattened(desc,v)", "flattened(a.b, x)", "flattened(a.b[].c)", "flattened(n)", "flattened(name:label=N)",
	"list", "list(name,tags)", "list[compact](name,zone)", "list[title=Things](name)", "list[always-display-title,title=T](name)",
	"list(desc)", "list(n)",
	"json(name,lab.env)", "json(tags[0], a.b[].c)", "json(x[1], a.e, z)", "json()", "json(name.basename(), tags.join('+'))",
	"json(a.b[0], a.b[].d)", "json(n)", "json(items[0].k)",
	"none",
	"table(projectId,URLPath,getHTTPResponseCode,a.name,b.name,name,snake_case,\"k.e.y\",'with space')",
	"flattened(projectId,a.name,b.name,\"k.e.y\",'with space')", "csv(a.name,b.name,name)",
	"table(w,e,c,s,t,u)", "table[box](w,e,c,u)", "table[box,title=全角タイトル](w,e)", "table(w:align=right,e:align=center,u:align=center)",
	"table[title=x](w,e)", "table[title=abcdefghijklmnopqrstuvwxyz0123](w)", "table[box,title=odd](u)", "table[box,title=even](u)",
	"csv(w,e,s,t,u)", "value(w,s,t)", "flattened(w,s,t,u)", "list(w,s)",
	"table(l,d,s,q,r,x)", "csv(l,d,s,q,r,x)", "value(l,d,s,q,r,x)", "value[quote](l,d,s,q,r,x)", "flattened(l,d,s,q,r,x)", "list(l,d)", "json(l,d)",
	"csv[delimiter=''](l)", "csv[separator=' '](q,r)", "csv[terminator=';'](s,x)",
	"table(long,id)", "table[box](long,id)", "table(id,long)", "table(long:align=right,id)",
	"table(l.len(), l.join(), l.join(sep='-', undefined='none'), d.join(), d.len(), s.join(' '), s.len(), x.basename(), nope.basename(undefined=N), nope.join(undefined=J), nope.len())",
	"table(name.basename().len(), name.len(), name.join('.'), name)",
	"value(d.join('&'), l.basename())",
	"table(l[0],l[-1],l[6],l[9],l[-9],d.m[1],d.n.x)", "json(l[0], l[-1], l[1], d.m[-1], d.n)", "json(l[], d)", "json(d.a, d.z)",
	"table(n:sort=1:reverse, name:sort=2)", "table(name:reverse, zone:reverse)", "table(zone:sort=2, name:sort=1)",
	"table(tags[], lab[])", "value(tags[0], tags[1], tags[2])",
	" table ( name , zone ) ", "table( name,zone )", "table[box] (name)", "table [box](name)", "table[ box](name)", "table[box ](name)",
	"table[box,](name)", "table[,box](name)", "table[](name)", "table[no-heading,box](name)", "table[title=\"a, b\"](name)",
	"table[title='x]y'](name)", "table[title=a\\tb](name)", "csv[separator=\\t](name,zone)", "value[terminator=\\n\\n](name)",
	"table(name:label=\"x y\")", "table(name : label=X)", "table(name:label=X :align=right)", "table(name:label=X:align=right)",
	"table(name,)", "table(,name)", "table(name zone)", "table(name", "table(name))", "table(name)(zone)", "table)", "table(",
	"table[box", "table[pad=x](name)", "table[pad=-1](name)", "table[pad=1.5](name)", "table[title=5](name)", "table[title='5'](name)",
	"table[box=1](name)", "table[title](name)", "csv[separator=1](name)", "table(name:label=5)", "table(name:label='5')",
	"table(name:align=middle)", "table(name:align)", "table(name:reverse=1)", "table(name:no-reverse)", "table(name:sort=0)",
	"table(name:sort=-1)", "table(name:sort=x)", "table(name:sort=1.5)", "table(name:foo=bar)", "table(name:wrap)",
	"table(name.basename(x,y))", "table(name.basename(undefined=u))", "table(name.len(1))", "table(name.join(sep=-,sep=+))",
	"table(name.join(',', 'u'))", "table(name.join(undefined='u', '+'))", "table(name.nope())", "table(name.basename().join())",
	"table(name.basename() .len())", "table(name.basename()x)", "table(name, NAME)", "table(name:label=N, N)", "table(zone, ZONE.x)",
	"table(a.name, b.name, name)", "table(name, name:label=again, name:align=right)", "table(name.len(), name:label=L)",
	"TABLE(name)", "Table(name)", "table1(name)", "ta-ble(name)", "'table'(name)", "\"table\"(name)", "table(\"name\")",
	"table('na' 'me')", "table(na\\me)", "table(lab.'a b')", "table(lab.\"a b\")",
	"table(name)[box]", "[box]table(name)", "table:(name)", "json(name)(zone)", "json[box](name)", "yaml[x]", "none(name)",
	"flattened[no-pad,separator='=']", "list[compact,title=T]", "value[quote,separator=';',delimiter=','](tags,name)",
}

func TestPrintMatchesGcloud(t *testing.T) {
	var cases []printCase
	for _, d := range diffDocs {
		for _, f := range diffFormats {
			cases = append(cases, printCase{f, d})
		}
	}
	var want []printResult
	gcloudtest.Run(t, printScript, cases, &want)
	compared := 0
	for i, c := range cases {
		w := want[i]
		got, err := printForDiff(c.Format, c.JSON)
		var unsupported *resource.UnsupportedError
		switch {
		case errors.As(err, &unsupported):
			t.Logf("skipped %s on %.30s: %v (gcloud: %.60q %s)", c.Format, c.JSON, err, w.Out, w.Error)
			continue
		case err != nil && w.Error != "":
			continue
		case err != nil:
			t.Errorf("%s on %.40s: %v; gcloud printed:\n%s", c.Format, c.JSON, err, w.Out)
			continue
		case w.Error != "":
			t.Errorf("%s on %.40s printed:\n%s\ngcloud failed: %s", c.Format, c.JSON, got, w.Error)
			continue
		}
		compared++
		if got != w.Out {
			t.Errorf("%s on %.60s:\n--- clix\n%s--- gcloud\n%s---", c.Format, strings.ReplaceAll(c.JSON, "\n", " "), got, w.Out)
		}
	}
	t.Logf("compared %d of %d cases with gcloud", compared, len(cases))
}

// printForDiff prints doc with format as the test compares it: json
// formats as canonical JSON of the projected data.
func printForDiff(format, doc string) (string, error) {
	f, err := Parse(format)
	if err != nil {
		return "", err
	}
	data, err := resource.Decode([]byte(doc))
	if err != nil {
		return "", err
	}
	if f.Name == "json" {
		v, ok := f.Project(data)
		if !ok {
			return "[]", nil
		}
		return string(appendCanonical(nil, v)), nil
	}
	var b bytes.Buffer
	err = f.Print(&b, data, Options{UTF8: true})
	return b.String(), err
}

// TestRandomFormatsMatchGcloud compares random format expressions, built
// from the pieces of the grammar, on the documents above.
func TestRandomFormatsMatchGcloud(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	pick := func(xs ...string) string { return xs[rng.IntN(len(xs))] }
	keys := []string{"name", "zone", "n", "f", "b", "tags", "tags[0]", "tags[-1]", "tags[]", "lab", "lab.env", "lab[]", "z",
		"desc", "id", "v", "a", "a.b", "a.b[0].c", "a.b[].c", "a.e", "x", "x[0]", "items[0].k", "nested.deep", "w", "e", "c", "s", "t", "u",
		"l", "d", "d.m", "q", "r", "long", "projectId", "parent.id", "missing", "selfLink", "URLPath"}
	key := func() string {
		k := pick(keys...)
		switch rng.IntN(8) {
		case 0:
			k += pick(".len()", ".basename()", ".join()", ".join(',')", ".basename().len()", ".join(sep=' ', undefined=U)")
		}
		switch rng.IntN(6) {
		case 0:
			k += pick(":label=L", ":label=''", ":align=right", ":align=center", ":sort=1", ":sort=2:reverse", ":reverse", ":label=X:sort=1")
		}
		return k
	}
	names := map[string][]string{
		"table":     {"box", "all-box", "no-heading", "title=T", "title='a long title here'", "pad=1", "pad=3"},
		"csv":       {"no-heading", "no-quote", "separator=';'", "delimiter='|'", "terminator=';'"},
		"value":     {"quote", "separator=','", "delimiter=' '", "terminator=';'"},
		"flattened": {"no-pad", "separator='='"},
		"list":      {"compact", "title=T", "always-display-title"},
		"json":      {},
	}
	var cases []printCase
	for i := 0; i < 4000; i++ {
		name := pick("table", "table", "csv", "value", "flattened", "list", "json")
		expr := name
		if attrs := names[name]; len(attrs) > 0 && rng.IntN(2) == 0 {
			var as []string
			for range 1 + rng.IntN(2) {
				as = append(as, attrs[rng.IntN(len(attrs))])
			}
			expr += "[" + strings.Join(as, ",") + "]"
		}
		if name != "flattened" && name != "list" && name != "json" || rng.IntN(2) == 0 {
			var ks []string
			for range 1 + rng.IntN(4) {
				ks = append(ks, key())
			}
			expr += "(" + strings.Join(ks, ", ") + ")"
		}
		cases = append(cases, printCase{expr, diffDocs[rng.IntN(len(diffDocs))]})
	}
	var want []printResult
	gcloudtest.Run(t, printScript, cases, &want)
	compared := 0
	for i, c := range cases {
		w := want[i]
		got, err := printForDiff(c.Format, c.JSON)
		var unsupported *resource.UnsupportedError
		switch {
		case errors.As(err, &unsupported):
			continue
		case err != nil && w.Error != "":
			continue
		case err != nil:
			t.Errorf("%s on %.40s: %v; gcloud printed:\n%s", c.Format, c.JSON, err, w.Out)
			continue
		case w.Error != "":
			t.Errorf("%s on %.40s printed:\n%s\ngcloud failed: %s", c.Format, c.JSON, got, w.Error)
			continue
		}
		compared++
		if got != w.Out {
			t.Errorf("%s on %.60s:\n--- clix\n%s--- gcloud\n%s---", c.Format, strings.ReplaceAll(c.JSON, "\n", " "), got, w.Out)
		}
	}
	t.Logf("compared %d of %d random cases with gcloud", compared, len(cases))
}
