package format_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
)

// project is a gcloud-like resource.
type project struct {
	ProjectID string            `json:"projectId" yaml:"id"`
	Name      string            `json:"name"`
	State     string            `json:"lifecycleState"`
	Number    int               `json:"projectNumber"`
	Labels    map[string]string `json:"labels,omitempty"`
	Created   string            `json:"createTime"`
}

var projects = []project{
	{"web-prod", "Web", "ACTIVE", 1001, map[string]string{"env": "prod"}, "2021-02-10T19:19:49Z"},
	{"web-test", "Web (test)", "ACTIVE", 1002, map[string]string{"env": "test"}, "2018-01-15T00:00:00Z"},
	{"old", "Café", "DELETE_REQUESTED", 99, nil, "2017-05-05T05:05:05Z"},
	{"api", "API", "ACTIVE", 7, map[string]string{"env": "prod"}, "2019-06-01T10:00:00Z"},
}

// listedJSON is a slice that encodes itself, so Write cannot rebuild it
// and keeps the JSON of the selected elements instead.
type listedJSON []project

func (l listedJSON) MarshalJSON() ([]byte, error) { return json.Marshal([]project(l)) }

// listApp builds "projects list" (with ListFlags), "projects describe" (one
// object), "projects none" (a nil interface), "projects raw" (listedJSON),
// "projects array" (an array) and "projects ptr" (a pointer to a slice).
func listApp(t *testing.T, ext bool) (*clix.App, *bytes.Buffer, *bool) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, v := range []string{"PROBE_FORMAT", "PROBE_FILTER", "PROBE_SORT_BY", "PROBE_LIMIT", "NO_COLOR", "CLICOLOR_FORCE"} {
		t.Setenv(v, "")
	}
	out := &bytes.Buffer{}
	app := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(&bytes.Buffer{}), clix.WithAppInteractive(clix.InteractiveNever))
	if ext {
		app.AddExtension(format.Extension{})
	}
	ran := new(bool)
	cmd := func(name string, data any) *clix.Command {
		return clix.NewCommand(name, format.WithListFlags(), clix.WithCommandRun(func(ctx *clix.Context) error {
			*ran = true
			return format.Write(ctx, data)
		}))
	}
	arr := [4]project(projects)
	group := clix.NewGroup("projects", "Manage projects",
		cmd("list", projects),
		cmd("describe", projects[0]),
		cmd("none", nil),
		cmd("raw", listedJSON(projects)),
		cmd("array", arr),
		cmd("ptr", &projects),
		cmd("maps", []map[string]any{{"n": 10}, {"n": 9}, {"n": "x"}}),
	)
	app.Root.AddCommand(group)
	return app, out, ran
}

func runList(t *testing.T, app *clix.App, out *bytes.Buffer, ran *bool, args ...string) (string, error) {
	t.Helper()
	out.Reset()
	*ran = false
	err := app.Run(context.Background(), args)
	return out.String(), err
}

// ids returns the projectIds in TSV text output (the first column).
func ids(text string) string {
	var out []string
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		if line != "" {
			out = append(out, strings.SplitN(line, "\t", 2)[0])
		}
	}
	return strings.Join(out, " ")
}

func TestListFlags(t *testing.T) {
	app, out, ran := listApp(t, true)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{nil, "web-prod web-test old api"},
		{[]string{"--filter", "lifecycleState=ACTIVE"}, "web-prod web-test api"},
		{[]string{"--filter=labels.env=prod"}, "web-prod api"},
		{[]string{"--filter", "name:cafe"}, "old"},
		{[]string{"--filter", "projectNumber>1000 OR project_id=old"}, "web-prod web-test old"},
		{[]string{"--filter", "createTime>=2018-01-15T00:00:00Z"}, "web-prod web-test api"},
		{[]string{"--filter", "-labels.env:*"}, "old"},
		{[]string{"--filter", ""}, "web-prod web-test old api"},
		{[]string{"--sort-by", "projectId"}, "api old web-prod web-test"},
		{[]string{"--sort-by", "~projectId"}, "web-test web-prod old api"},
		{[]string{"--sort-by", "labels.env,~name"}, "old web-prod api web-test"},
		{[]string{"--sort-by", "projectNumber"}, "web-prod web-test api old"}, // as text, like gcloud
		{[]string{"--limit", "2"}, "web-prod web-test"},
		{[]string{"--limit", "unlimited"}, "web-prod web-test old api"},
		{[]string{"--limit", "10"}, "web-prod web-test old api"},
		// Filter, then sort, then limit.
		{[]string{"--limit", "1", "--sort-by", "~createTime", "--filter", "labels.env=prod"}, "web-prod"},
		{[]string{"--filter", "labels.env=prod", "--sort-by", "createTime", "--limit", "1"}, "api"},
		{[]string{"--filter", "name:nope"}, ""},
	} {
		got, err := runList(t, app, out, ran, append([]string{"projects", "list"}, tc.args...)...)
		if err != nil {
			t.Errorf("%q: %v", tc.args, err)
			continue
		}
		if ids(got) != tc.want {
			t.Errorf("%q printed %q, want %q", tc.args, ids(got), tc.want)
		}
	}
}

// The list flags apply before every output flag.
func TestListFlagsWithOutputFlags(t *testing.T) {
	app, out, ran := listApp(t, true)
	got, err := runList(t, app, out, ran, "projects", "list", "--filter", "labels.env=prod", "--format", "json")
	var list []project
	if err != nil || json.Unmarshal([]byte(got), &list) != nil || len(list) != 2 || list[1].ProjectID != "api" {
		t.Errorf("--format json: %v %q", err, got)
	}
	got, err = runList(t, app, out, ran, "projects", "list", "--sort-by", "projectId", "--limit", "2", "--json", "projectId")
	want := "[\n  {\n    \"projectId\": \"api\"\n  },\n  {\n    \"projectId\": \"old\"\n  }\n]\n"
	if err != nil || got != want {
		t.Errorf("--json: %v\n%s", err, got)
	}
	got, err = runList(t, app, out, ran, "projects", "list", "--filter", "lifecycleState!=ACTIVE", "--jq", ".[].name")
	if err != nil || got != "Café\n" {
		t.Errorf("--jq: %v %q", err, got)
	}
	got, err = runList(t, app, out, ran, "projects", "list", "--limit", "1", "--template", "{{range .}}{{.projectId}}{{end}}")
	if err != nil || got != "web-prod" {
		t.Errorf("--template: %v %q", err, got)
	}
	// YAML keeps the struct's own field names: the elements stay structs.
	full, _ := runList(t, app, out, ran, "projects", "list", "--format", "yaml")
	limited, err := runList(t, app, out, ran, "projects", "list", "--format", "yaml", "--limit", "unlimited", "--filter", "name:*")
	if err != nil || full != limited || !strings.Contains(full, "id: web-prod") {
		t.Errorf("--format yaml: %v\n%s\n%s", err, full, limited)
	}
	// An empty result keeps the struct's fields for --json.
	_, err = runList(t, app, out, ran, "projects", "list", "--filter", "name:nope", "--json")
	if err == nil || !strings.Contains(err.Error(), "lifecycleState") {
		t.Errorf("--json on an empty result: %v", err)
	}
	// Without the format extension, Write still applies the list flags.
	plain, out2, ran2 := listApp(t, false)
	got, err = runList(t, plain, out2, ran2, "projects", "list", "--filter", "projectId:web", "--limit", "1")
	if err != nil || ids(got) != "web-prod" {
		t.Errorf("without the extension: %v %q", err, got)
	}
}

// Slices, arrays, pointers to slices and lists that encode themselves.
func TestListFlagsDataShapes(t *testing.T) {
	app, out, ran := listApp(t, true)
	for _, cmd := range []string{"list", "array", "ptr", "raw"} {
		got, err := runList(t, app, out, ran, "projects", cmd, "--filter", "labels.env=prod", "--sort-by", "~projectNumber", "--format", "json")
		var list []project
		if err != nil || json.Unmarshal([]byte(got), &list) != nil || len(list) != 2 || list[0].ProjectID != "api" || list[1].ProjectID != "web-prod" {
			t.Errorf("%s: %v %s", cmd, err, got)
		}
	}
	got, err := runList(t, app, out, ran, "projects", "raw", "--limit", "1")
	if err != nil || ids(got) != "web-prod" {
		t.Errorf("raw text: %v %q", err, got)
	}
	got, err = runList(t, app, out, ran, "projects", "maps", "--sort-by", "n")
	if err != nil || got != "10\n9\nx\n" {
		t.Errorf("maps: %v %q", err, got)
	}
	// nil prints nothing, as without the flags.
	if got, err := runList(t, app, out, ran, "projects", "none", "--limit", "1"); err != nil || got != "" {
		t.Errorf("none: %v %q", err, got)
	}
	// One object is not a list.
	_, err = runList(t, app, out, ran, "projects", "describe", "--filter", "name:x", "--limit", "1")
	if err == nil || clix.ExitCode(err) != 2 || err.Error() != "--filter, --limit apply to a list, and this command prints a single object" {
		t.Errorf("describe: %v", err)
	}
}

func TestListFlagsErrors(t *testing.T) {
	app, out, ran := listApp(t, true)
	for _, tc := range []struct {
		args []string
		msg  string
	}{
		{[]string{"--filter", "a=1 AND b=1 OR c=1"}, "Parenthesis grouping is required"},
		{[]string{"--filter", "name~web"}, "the ~ operator (a Python regular expression) is not supported in clix --filter"},
		{[]string{"--filter", "name"}, "global restriction"},
		{[]string{"--sort-by", "a,,b"}, "empty field"},
		{[]string{"--sort-by", "name:x"}, "Unexpected tokens"},
		{[]string{"--sort-by", "*name"}, "is not supported in clix --sort-by"},
		{[]string{"--limit", "0"}, `"0" is not a positive integer or unlimited`},
		{[]string{"--limit", "-1"}, "not a positive integer"},
		{[]string{"--limit", "x"}, "not a positive integer"},
	} {
		got, err := runList(t, app, out, ran, append([]string{"projects", "list"}, tc.args...)...)
		if err == nil || clix.ExitCode(err) != 2 || !strings.Contains(err.Error(), tc.msg) {
			t.Errorf("%q: %v (exit %d), want a usage error containing %q", tc.args, err, clix.ExitCode(err), tc.msg)
		}
		if *ran || got != "" {
			t.Errorf("%q: the command ran (%v) or printed %q", tc.args, *ran, got)
		}
	}
	// A filter clix cannot evaluate on the data as gcloud would is a usage
	// error from Write, before anything is printed.
	got, err := runList(t, app, out, ran, "projects", "list", "--filter", "createTime:2021")
	var unsupported interface{ Error() string }
	if err == nil || clix.ExitCode(err) != 2 || !errors.As(err, &unsupported) || !strings.Contains(err.Error(), "the : operator on createTime, which holds dates and times, is not supported in clix --filter") || got != "" {
		t.Errorf("createTime:2021: %v %q", err, got)
	}
}

// The list flags come from the command line only.
func TestListFlagsCommandLineOnly(t *testing.T) {
	app, out, ran := listApp(t, true)
	t.Setenv("PROBE_FILTER", "name:nope")
	t.Setenv("PROBE_LIMIT", "1")
	t.Setenv("PROBE_SORT_BY", "projectId")
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "probe"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "probe", "config.yaml"), []byte("filter: name:nope\nlimit: 1\nsort-by: projectId\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := runList(t, app, out, ran, "projects", "list")
	if err != nil || ids(got) != "web-prod web-test old api" {
		t.Errorf("env and config applied: %v %q", err, got)
	}
}

func TestListFlagsRegistration(t *testing.T) {
	cmd := clix.NewCommand("list")
	format.ListFlags(cmd)
	format.ListFlags(cmd) // a second call does nothing
	for _, name := range []string{"filter", "sort-by", "limit"} {
		f := cmd.Flags.Lookup(name)
		if f == nil || !f.Ambient || !f.CommandLineOnly {
			t.Errorf("--%s: %+v", name, f)
		}
	}
	if err := cmd.Flags.Err(); err != nil {
		t.Errorf("Err: %v", err)
	}
	format.ListFlags(nil)
	bare := &clix.Command{Name: "bare"}
	format.ListFlags(bare)
	if bare.Flags.Lookup("limit") == nil {
		t.Error("a command without a flag set")
	}
}

// A flag the command already has keeps its meaning, and Write ignores it.
func TestListFlagsKeepCommandFlags(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out := &bytes.Buffer{}
	app := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(&bytes.Buffer{}), clix.WithAppInteractive(clix.InteractiveNever))
	var pageLimit int
	var gotFilter string
	list := clix.NewCommand("list")
	list.Flags.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "limit"}, Value: &pageLimit})
	list.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "filter"}, Value: &gotFilter})
	format.ListFlags(list)
	list.Run = func(ctx *clix.Context) error { return format.Write(ctx, projects) }
	app.Root.AddCommand(list)
	if err := app.Run(context.Background(), []string{"list", "--limit", "1", "--filter", "x", "--sort-by", "~projectId"}); err != nil {
		t.Fatal(err)
	}
	if pageLimit != 1 || gotFilter != "x" || ids(out.String()) != "web-test web-prod old api" {
		t.Errorf("limit %d, filter %q, output %q", pageLimit, gotFilter, ids(out.String()))
	}
	if err := app.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

// The list flags are Ambient: with them, Run still prompts for missing
// required flags.
func TestListFlagsAreAmbient(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PROBE_DISABLE_PROMPTS", "")
	out := &bytes.Buffer{}
	app := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(&bytes.Buffer{}),
		clix.WithAppIn(strings.NewReader("p1\n")), clix.WithAppInteractive(clix.InteractiveAlways))
	var parent string
	list := clix.NewCommand("list", format.WithListFlags(), clix.WithCommandRun(func(ctx *clix.Context) error {
		return format.Write(ctx, projects)
	}))
	list.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "parent", Required: true}, Value: &parent})
	app.Root.AddCommand(list)
	if err := app.Run(context.Background(), []string{"list", "--filter", "projectId:web", "--sort-by", "name", "--limit", "1"}); err != nil {
		t.Fatal(err)
	}
	if parent != "p1" || ids(out.String()) != "web-prod" {
		t.Errorf("parent %q, output %q", parent, out.String())
	}
}

func TestListFlagsHelp(t *testing.T) {
	app, out, ran := listApp(t, true)
	got, err := runList(t, app, out, ran, "projects", "list", "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--filter", "--sort-by", "--limit", "~FIELD"} {
		if !strings.Contains(got, want) {
			t.Errorf("help lacks %q:\n%s", want, got)
		}
	}
}

func TestFiltersTopic(t *testing.T) {
	for _, want := range []string{"key:pattern", "AND", "--sort-by", "--limit", "not supported"} {
		if !strings.Contains(format.FiltersTopic, want) {
			t.Errorf("FiltersTopic lacks %q", want)
		}
	}
}
