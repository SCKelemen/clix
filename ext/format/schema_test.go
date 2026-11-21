package format_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
	"go.yaml.in/yaml/v3"
)

// sparse records, as an encoder that omits default values writes them:
// the second record has no stars, owner, tags or labels.
var sparseRecords = []json.RawMessage{
	json.RawMessage(`{"name":"clix","stars":42,"owner":{"login":"sam","id":1},"tags":["cli","go"],"labels":{"tier":"core"}}`),
	json.RawMessage(`{"name":"jq"}`),
}

var recordSchema = format.Fields{
	{Name: "name", Zero: ""},
	{Name: "stars", Zero: 0},
	{Name: "owner", Nullable: true, Fields: []format.Field{{Name: "login", Zero: ""}, {Name: "id", Zero: 0}}},
	{Name: "tags", Zero: []string{}},
	{Name: "labels", Zero: map[string]string{}},
}

// The canonical document of sparseRecords under recordSchema.
const canonicalRecords = `[
  {
    "name": "clix",
    "stars": 42,
    "owner": {
      "login": "sam",
      "id": 1
    },
    "tags": [
      "cli",
      "go"
    ],
    "labels": {
      "tier": "core"
    }
  },
  {
    "name": "jq",
    "stars": 0,
    "owner": null,
    "tags": [],
    "labels": {}
  }
]
`

// schemaApp builds an app with every output option on, whose commands
// print data with format.Write: records (a schema-wrapped list), one (a
// schema-wrapped record) and none (an empty schema-wrapped list).
func schemaApp(t *testing.T, ext format.Extension, commands map[string]any) (*clix.App, *bytes.Buffer, *bool) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, v := range []string{"PROBE_FORMAT", "NO_COLOR", "CLICOLOR_FORCE"} {
		t.Setenv(v, "")
	}
	out := &bytes.Buffer{}
	app := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(&bytes.Buffer{}), clix.WithAppInteractive(clix.InteractiveNever))
	app.AddExtension(ext)
	ran := new(bool)
	for name, data := range commands {
		app.Root.AddCommand(clix.NewCommand(name, format.WithListFlags(), clix.WithCommandRun(func(ctx *clix.Context) error {
			*ran = true
			return format.Write(ctx, data)
		})))
	}
	return app, out, ran
}

func schemaCommands() map[string]any {
	return map[string]any{
		"records": format.Document{Value: sparseRecords, Schema: recordSchema},
		"one":     format.Document{Value: sparseRecords[1], Schema: recordSchema},
		"none":    format.Document{Value: []json.RawMessage{}, Schema: recordSchema},
	}
}

func mustRun(t *testing.T, app *clix.App, out *bytes.Buffer, ran *bool, args ...string) string {
	t.Helper()
	got, err := runWrite(t, app, out, ran, args...)
	if err != nil {
		t.Fatalf("%q: %v", args, err)
	}
	return got
}

// decodeLikeTemplates decodes JSON as --template and row templates see
// it: objects as maps, whole numbers as int64.
func decodeLikeTemplates(t *testing.T, doc string) any {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(doc))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	var numbers func(any) any
	numbers = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			for k, e := range x {
				x[k] = numbers(e)
			}
		case []any:
			for i, e := range x {
				x[i] = numbers(e)
			}
		case json.Number:
			if i, err := strconv.ParseInt(string(x), 10, 64); err == nil {
				return i
			}
			f, _ := x.Float64()
			return f
		}
		return v
	}
	return numbers(v)
}

// One schema-wrapped document is the same data under every output flag.
func TestSchemaOneDocumentForEveryFlag(t *testing.T) {
	app, out, ran := schemaApp(t, format.Extension{RowTemplates: true, Projections: true}, schemaCommands())

	plain := mustRun(t, app, out, ran, "records", "--format", "json")
	if plain != canonicalRecords {
		t.Fatalf("--format json:\n%s\nwant:\n%s", plain, canonicalRecords)
	}
	if got := mustRun(t, app, out, ran, "records", "--jq", "."); got != plain {
		t.Errorf("--jq . differs from --format json:\n%s", got)
	}
	if got := mustRun(t, app, out, ran, "records", "--json", "name,stars,owner,tags,labels"); got != plain {
		t.Errorf("--json with every field differs from --format json:\n%s", got)
	}

	// --template and row templates see the decoded canonical document.
	decoded := decodeLikeTemplates(t, plain)
	if got, want := mustRun(t, app, out, ran, "records", "--template", "{{.}}"), fmt.Sprint(decoded); got != want {
		t.Errorf("--template {{.}} = %q, want %q", got, want)
	}
	var wantRows, wantJSONRows strings.Builder
	for _, record := range decoded.([]any) {
		fmt.Fprintln(&wantRows, record)
		b, _ := json.Marshal(record)
		fmt.Fprintln(&wantJSONRows, string(b))
	}
	if got := mustRun(t, app, out, ran, "records", "--format", "{{.}}"); got != wantRows.String() {
		t.Errorf("row template {{.}} = %q, want %q", got, wantRows.String())
	}
	if got := mustRun(t, app, out, ran, "records", "--format", "{{json .}}"); got != wantJSONRows.String() {
		t.Errorf("row template {{json .}} = %q, want %q", got, wantJSONRows.String())
	}

	// YAML holds the same values.
	var fromYAML, fromJSON any
	if err := yaml.Unmarshal([]byte(mustRun(t, app, out, ran, "records", "--format", "yaml")), &fromYAML); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(plain), &fromJSON); err != nil {
		t.Fatal(err)
	}
	yamlJSON, _ := json.Marshal(fromYAML)
	jsonJSON, _ := json.Marshal(fromJSON)
	if string(yamlJSON) != string(jsonJSON) {
		t.Errorf("--format yaml holds %s, want %s", yamlJSON, jsonJSON)
	}

	// Text, gcloud projections and the list flags read it too.
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"records"}, "clix\t42\t{\"id\":1,\"login\":\"sam\"}\tcli, go\t{\"tier\":\"core\"}\njq\t0\t\t\t{}\n"},
		{[]string{"records", "--format", "value(name,stars,tags.len())"}, "clix\t42\t2\njq\t0\t0\n"},
		{[]string{"records", "--filter", "stars=0", "--format", "value(name)"}, "jq\n"},
		{[]string{"records", "--sort-by", "stars", "--limit", "1", "--jq", ".[].name"}, "jq\n"},
		{[]string{"records", "--jq", `.[1] | .stars == 0 and .owner == null and .tags == []`}, "true\n"},
		{[]string{"records", "--format", `{{.name}}|{{.stars}}|{{len .tags}}`}, "clix|42|2\njq|0|0\n"},
		{[]string{"records", "--template", `{{range .}}{{.name}}:{{.stars}} {{end}}`}, "clix:42 jq:0 "},
		{[]string{"one", "--format", "json"}, "{\n  \"name\": \"jq\",\n  \"stars\": 0,\n  \"owner\": null,\n  \"tags\": [],\n  \"labels\": {}\n}\n"},
		{[]string{"one", "--format", "{{.name}} {{.stars}}"}, "jq 0\n"},
	} {
		if got := mustRun(t, app, out, ran, tc.args...); got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.args, got, tc.want)
		}
	}
}

// A missing field gets its Zero, or null when it is Nullable; a nested
// object is filled too, and a nil Zero with nested fields is an object
// of their zero values. Fields the record has, even null, are kept, and
// a filled field goes after the field declared before it.
func TestSchemaFillsMissingFields(t *testing.T) {
	schema := format.Fields{
		{Name: "id", Zero: 0},
		{Name: "name", Zero: ""},
		{Name: "enabled", Zero: false},
		{Name: "count", Zero: int64(0)},
		{Name: "nickname", Nullable: true, Zero: "ignored"},
		{Name: "spec", Fields: []format.Field{{Name: "replicas", Zero: 1}, {Name: "image", Nullable: true}}},
		{Name: "items", Zero: []any{}, Fields: []format.Field{{Name: "key", Zero: ""}, {Name: "value", Nullable: true}}},
		{Name: "extra"},
	}
	value := json.RawMessage(`{"name":"web","enabled":null,"note":"kept","spec":{"image":"nginx"},"items":[{"key":"a"},{"value":2},"plain",null]}`)
	var got bytes.Buffer
	if err := clix.FormatData(&got, format.Document{Value: value, Schema: schema}, clix.FormatJSON); err != nil {
		t.Fatal(err)
	}
	want := `{
  "id": 0,
  "name": "web",
  "enabled": null,
  "count": 0,
  "nickname": null,
  "note": "kept",
  "spec": {
    "replicas": 1,
    "image": "nginx"
  },
  "items": [
    {
      "key": "a",
      "value": null
    },
    {
      "key": "",
      "value": 2
    },
    "plain",
    null
  ],
  "extra": null
}
`
	if got.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", got.String(), want)
	}

	// A missing object with nested fields and no Zero is an object of
	// zero values; a Nullable one is null.
	got.Reset()
	if err := clix.FormatData(&got, format.Document{Value: map[string]any{}, Schema: schema[5:6]}, clix.FormatJSON); err != nil {
		t.Fatal(err)
	}
	if want := "{\n  \"spec\": {\n    \"replicas\": 1,\n    \"image\": null\n  }\n}\n"; got.String() != want {
		t.Errorf("missing spec: got %q, want %q", got.String(), want)
	}
}

// A Document prints its canonical document wherever clix prints it, also
// through clix.FormatData and inside other values.
func TestDocumentFormatsEverywhere(t *testing.T) {
	doc := format.Document{Value: sparseRecords[1], Schema: recordSchema}
	var b bytes.Buffer
	if err := clix.FormatData(&b, doc, clix.FormatYAML); err != nil {
		t.Fatal(err)
	}
	if want := "labels: {}\nname: jq\nowner: null\nstars: 0\ntags: []\n"; b.String() != want {
		t.Errorf("YAML: got %q, want %q", b.String(), want)
	}
	b.Reset()
	if err := clix.FormatData(&b, doc, clix.FormatText); err != nil {
		t.Fatal(err)
	}
	if want := "labels = \nname = jq\nowner = \nstars = 0\ntags = \n"; b.String() != want {
		t.Errorf("text: got %q, want %q", b.String(), want)
	}
	b.Reset()
	if err := clix.FormatData(&b, map[string]any{"item": doc}, clix.FormatJSON); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), `"stars": 0`) {
		t.Errorf("nested Document: %s", b.String())
	}
	// A nil Schema prints Value as it is.
	b.Reset()
	if err := clix.FormatData(&b, format.Document{Value: sparseRecords[1]}, clix.FormatJSON); err != nil {
		t.Fatal(err)
	}
	if want := "{\n  \"name\": \"jq\"\n}\n"; b.String() != want {
		t.Errorf("nil Schema: got %q, want %q", b.String(), want)
	}
}

type schemaRepo struct {
	Name  string `json:"name"`
	Stars int    `json:"stars,omitempty"`
	Note  string `json:"note,omitempty"`
}

func (*schemaRepo) Fields() []format.Field {
	return []format.Field{{Name: "name", Zero: ""}, {Name: "stars", Zero: 0}, {Name: "note", Nullable: true}}
}

// A result whose type, or element type, implements Schema needs no
// Document.
func TestSchemaFromTheResultType(t *testing.T) {
	repos := []schemaRepo{{Name: "clix", Stars: 42, Note: "kit"}, {Name: "jq"}}
	app, out, ran := schemaApp(t, format.Extension{}, map[string]any{
		"list":  repos,
		"ptrs":  []*schemaRepo{&repos[1]},
		"get":   repos[1],
		"empty": []schemaRepo{},
	})
	if got, want := mustRun(t, app, out, ran, "list", "--jq", ".[1]"), "{\n  \"name\": \"jq\",\n  \"stars\": 0,\n  \"note\": null\n}\n"; got != want {
		t.Errorf("list: got %q, want %q", got, want)
	}
	if got, want := mustRun(t, app, out, ran, "ptrs", "--jq", ".[0].stars"), "0\n"; got != want {
		t.Errorf("ptrs: got %q, want %q", got, want)
	}
	if got, want := mustRun(t, app, out, ran, "get", "--format", "json"), "{\n  \"name\": \"jq\",\n  \"stars\": 0,\n  \"note\": null\n}\n"; got != want {
		t.Errorf("get: got %q, want %q", got, want)
	}
	if got, want := mustRun(t, app, out, ran, "empty", "--json", "stars"), "[]\n"; got != want {
		t.Errorf("empty: got %q, want %q", got, want)
	}
}

// Without a schema, a --json selection gives null for a selected key an
// object lacks, the same for every producer, in a consistent place.
func TestJSONSelectionMissingKeyIsNull(t *testing.T) {
	app, out, ran := writeApp(t)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"maps", "--json", "a,c"}, "[\n  {\n    \"a\": \"x\",\n    \"c\": null\n  },\n  {\n    \"a\": null,\n    \"c\": true\n  }\n]\n"},
		{[]string{"maps", "--json", "c,a"}, "[\n  {\n    \"a\": \"x\",\n    \"c\": null\n  },\n  {\n    \"a\": null,\n    \"c\": true\n  }\n]\n"},
		{[]string{"get", "--json", "note,name"}, "{\n  \"name\": \"clix\",\n  \"note\": null\n}\n"},
		{[]string{"list", "--json", "owner.login,owner.nickname", "--jq", ".[] | .owner"}, "{\n  \"login\": \"sam\",\n  \"nickname\": null\n}\nnull\n"},
		{[]string{"list", "--json", "labels.name,labels.size", "--jq", ".[0].labels[0]"}, "{\n  \"name\": \"cli\",\n  \"size\": null\n}\n"},
		{[]string{"list", "--json", "note", "--template", "{{range .}}{{.note}};{{end}}"}, "<no value>;<no value>;"},
	} {
		if got := mustRun(t, app, out, ran, tc.args...); got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.args, got, tc.want)
		}
	}
}

// With a schema, --json takes its fields, also for an empty list, and
// checks paths into fields whose nested fields it declares.
func TestSchemaValidatesSelection(t *testing.T) {
	app, out, ran := schemaApp(t, format.Extension{}, schemaCommands())
	if got, want := mustRun(t, app, out, ran, "none", "--json", "stars,owner.login"), "[]\n"; got != want {
		t.Errorf("none: got %q, want %q", got, want)
	}
	if got, want := mustRun(t, app, out, ran, "records", "--json", "labels.tier", "--jq", ".[].labels"), "{\n  \"tier\": \"core\"\n}\n{\n  \"tier\": null\n}\n"; got != want {
		t.Errorf("a map's keys are data: got %q, want %q", got, want)
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"none", "--json"}, "specify one or more comma-separated fields for --json:\n  labels\n  name\n  owner\n  stars\n  tags"},
		{[]string{"none", "--json", "nope"}, "unknown field for --json: \"nope\"\navailable fields:\n  labels\n  name\n  owner\n  stars\n  tags"},
		{[]string{"records", "--json", "owner.logn"}, "unknown field for --json: \"owner.logn\"\navailable fields:\n  owner.id\n  owner.login"},
	} {
		_, err := runWrite(t, app, out, ran, tc.args...)
		if err == nil || clix.ExitCode(err) != 2 || err.Error() != tc.want {
			t.Errorf("%q: err = %v, want %q", tc.args, err, tc.want)
		}
	}
}

// Row templates fail on a field the record lacks by default, and print it
// as nothing with MissingZero. A schema's fields are never missing.
func TestRowTemplateMissingFields(t *testing.T) {
	records := []map[string]any{
		{"name": "web", "status": "Up", "owner": map[string]any{"login": "sam"}, "tags": []string{"a", "b"}},
		{"name": "db", "owner": nil},
	}
	commands := map[string]any{"ps": records, "empty": []map[string]any{}}
	for k, v := range schemaCommands() {
		commands[k] = v
	}

	strict, out, ran := schemaApp(t, format.Extension{RowTemplates: true}, commands)
	for _, tc := range []struct {
		args     []string
		contains string
	}{
		{[]string{"ps", "--format", "{{.name}} {{.status}}"}, `row 2: template: row:1:12: executing "row" at <.status>: map has no entry for key "status"`},
		{[]string{"ps", "--format", "{{.owner.login}}"}, "row 2"},
		// With a schema, an unknown field fails even with no rows.
		{[]string{"none", "--format", "{{.name}} {{.nmae}}"}, "unknown field for --format: \"nmae\"\navailable fields:\n  labels\n  name\n  owner\n  stars\n  tags"},
		{[]string{"none", "--format", "{{range .tags}}{{.}}{{end}}{{$.nope}}"}, `unknown field for --format: "nope"`},
	} {
		got, err := runWrite(t, strict, out, ran, tc.args...)
		if err == nil || clix.ExitCode(err) != 2 || !strings.Contains(err.Error(), tc.contains) || got != "" {
			t.Errorf("%q: stdout=%q, err=%v; want an error containing %q", tc.args, got, err, tc.contains)
		}
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"records", "--format", "{{.name}} {{.stars}} {{.owner}}"}, "clix 42 map[id:1 login:sam]\njq 0 <no value>\n"},
		{[]string{"none", "--format", "{{.name}}"}, ""},
		{[]string{"empty", "--format", "{{.anything}}"}, ""},
		{[]string{"none", "--format", "{{range .tags}}{{.anything}}{{end}}"}, ""},
	} {
		if got := mustRun(t, strict, out, ran, tc.args...); got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.args, got, tc.want)
		}
	}

	lenient, out, ran := schemaApp(t, format.Extension{RowTemplates: true, MissingFields: format.MissingZero}, commands)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"ps", "--format", "{{.name}}|{{.status}}|{{.nope.deeper}}"}, "web|Up|\ndb||\n"},
		{[]string{"ps", "--format", "{{.owner.login}}|{{$.owner.login}}|{{(.owner).login}}"}, "sam|sam|sam\n||\n"},
		{[]string{"ps", "--format", `{{if .status}}up{{else}}down{{end}} {{upper .status}} {{lower .status}} {{join .tags ","}} {{range .tags}}<{{.}}>{{end}} {{json .status}}`}, "up UP up a,b <a><b> \"Up\"\ndown     null\n"},
		{[]string{"ps", "--format", `{{with .owner}}{{.login}}{{else}}none{{end}} {{range split .status "p"}}[{{.}}]{{end}} {{$s := .status}}{{$s}}`}, "sam [U][] Up\nnone  \n"},
		{[]string{"ps", "--format", `{{define "who"}}{{.login}}{{end}}{{template "who" .owner}}!`}, "sam!\n!\n"},
		{[]string{"ps", "--format", "table {{.name}}\\t{{.status}}"}, "NAME   STATUS\nweb    Up\ndb     \n"},
		{[]string{"records", "--format", "{{.name}} {{.stars}} {{.owner}} {{.owner.login}}"}, "clix 42 map[id:1 login:sam] sam\njq 0  \n"},
		{[]string{"none", "--format", "{{.nmae}}"}, ""},
	} {
		if got := mustRun(t, lenient, out, ran, tc.args...); got != tc.want {
			t.Errorf("MissingZero %q: got %q, want %q", tc.args, got, tc.want)
		}
	}
	// A field of a value that is not an object is still an error.
	if _, err := runWrite(t, lenient, out, ran, "ps", "--format", "{{.name.first}}"); err == nil || clix.ExitCode(err) != 2 || !strings.Contains(err.Error(), "can't evaluate field first in type string") {
		t.Errorf("field of a string: err = %v", err)
	}
}

// Without a schema, and with a nil Schema, output is what it was: Write
// prints a Document with a nil Schema exactly as its Value, under every
// flag.
func TestNoSchemaIsUnchanged(t *testing.T) {
	commands := map[string]any{"plain": wRepos, "wrapped": format.Document{Value: wRepos}}
	app, out, ran := schemaApp(t, format.Extension{RowTemplates: true, Projections: true}, commands)
	for _, args := range [][]string{
		{},
		{"--format", "json"},
		{"--format", "yaml"},
		{"--format", "table(name,stars)"},
		{"--format", "{{.name}} {{.stars}}"},
		{"--json", "name,owner.login"},
		{"--jq", ".[].name"},
		{"--template", "{{range .}}{{.name}}{{end}}"},
		{"--filter", "stars>10", "--format", "value(name)"},
	} {
		plain := mustRun(t, app, out, ran, append([]string{"plain"}, args...)...)
		if wrapped := mustRun(t, app, out, ran, append([]string{"wrapped"}, args...)...); wrapped != plain {
			t.Errorf("%q: Document without a schema printed %q, want %q", args, wrapped, plain)
		}
	}
	if got := mustRun(t, app, out, ran, "plain", "--format", "json"); got != formatJSON(t, wRepos) {
		t.Errorf("--format json changed: %q", got)
	}
}

type badZero struct{}

func (badZero) MarshalJSON() ([]byte, error) { return nil, fmt.Errorf("no JSON") }

// A schema with a field without a name, a field declared twice or a Zero
// that cannot be encoded is an error, not output.
func TestSchemaErrors(t *testing.T) {
	for _, tc := range []struct {
		schema format.Fields
		want   string
	}{
		{format.Fields{{Name: ""}}, `format: schema field with no name under ""`},
		{format.Fields{{Name: "a"}, {Name: "a"}}, `format: schema field "a" declared twice`},
		{format.Fields{{Name: "a", Fields: []format.Field{{Name: "b"}, {Name: "b"}}}}, `format: schema field "a.b" declared twice`},
		{format.Fields{{Name: "a", Zero: badZero{}}}, `format: zero value of schema field "a"`},
	} {
		app, out, ran := schemaApp(t, format.Extension{}, map[string]any{"get": format.Document{Value: map[string]any{}, Schema: tc.schema}})
		got, err := runWrite(t, app, out, ran, "get")
		if err == nil || !strings.Contains(err.Error(), tc.want) || got != "" {
			t.Errorf("%v: stdout=%q, err=%v; want %q", tc.schema, got, err, tc.want)
		}
	}
}
