package format_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
)

func TestRowFormat(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PROBE_FORMAT", "")
	out := new(bytes.Buffer)
	app := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(new(bytes.Buffer)), clix.WithAppInteractive(clix.InteractiveNever))
	app.AddExtension(format.Extension{RowTemplates: true, Projections: true})
	ran := false
	containers := []map[string]any{
		{"Names": "cache-service", "Status": "Up", "Ports": "9010/tcp", "Tags": []string{"one", "two"}},
		{"Names": "worker", "Status": "Exited", "Ports": "", "Tags": []string{}},
	}
	for name, value := range map[string]any{"ps": containers, "inspect": containers[0], "empty": containers[:0], "missing": []any{containers[0], map[string]string{"Names": "other"}}} {
		app.Root.AddCommand(clix.NewCommand(name, format.WithListFlags(), clix.WithCommandRun(func(ctx *clix.Context) error {
			ran = true
			return format.Write(ctx, value)
		})))
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"ps", "--format", "{{.Names}} {{.Status}} {{.Ports}}"}, "cache-service Up 9010/tcp\nworker Exited \n"},
		{[]string{"ps", "--filter", "Names=cache-service", "--format", "{{.Names}} {{.Status}} {{.Ports}}"}, "cache-service Up 9010/tcp\n"},
		{[]string{"ps", "--sort-by", "~Names", "--limit", "1", "--format", "{{.Names}}"}, "worker\n"},
		{[]string{"inspect", "--format", `{{upper .Status}}\t{{join .Tags ","}}|{{json .Tags}}`}, "UP\tone,two|[\"one\",\"two\"]\n"},
		{[]string{"empty", "--format", "{{.Names}}"}, ""},
		{[]string{"inspect", "--format", `{{join .Tags "\n"}}`}, "one\ntwo\n"},
		{[]string{"inspect", "--format", `{{printf "%s" "\\n"}}{{with .Tags}}{{range .}}\t{{.}}{{end}}{{end}}`}, "\\n\tone\ttwo\n"},
		{[]string{"inspect", "--format", `{{range split "" ","}}{{if .}}unused{{else}}else\t{{end}}{{end}}`}, "else\t\n"},
		{[]string{"inspect", "--format", `{{define "line"}}{{if .Tags}}tags\t{{printf "%s\n" .Names}}{{else}}empty\n{{end}}{{end}}{{template "line" .}}`}, "tags\tcache-service\n\n"},
		{[]string{"empty", "--format", `table {{.Names}}\t{{.Status}}`}, "NAMES   STATUS\n"},
		{[]string{"ps", "--format", `table {{.Names}}\t{{.Status}}`}, "NAMES           STATUS\ncache-service   Up\nworker          Exited\n"},
		{[]string{"ps", "--format", "value(Names)"}, "cache-service\nworker\n"},
		{[]string{"ps", "--template", "{{len .}}"}, "2"},
	} {
		got, err := runWrite(t, app, out, &ran, tc.args...)
		if err != nil || got != tc.want {
			t.Errorf("%q: got %q, %v; want %q", tc.args, got, err, tc.want)
		}
	}
	for _, tc := range []struct {
		args     []string
		contains string
		ran      bool
	}{
		{[]string{"ps", "--format", "{{.Names"}, "unclosed", false},
		{[]string{"ps", "--format", "table {{len .Tags}}"}, "direct field selectors", false},
		{[]string{"missing", "--format", "{{.Status}}"}, "row 2", true},
		{[]string{"ps", "--json=Names", "--format", "{{.Names}}"}, "--format {{.Names}}", true},
	} {
		got, err := runWrite(t, app, out, &ran, tc.args...)
		if err == nil || clix.ExitCode(err) != 2 || !strings.Contains(err.Error(), tc.contains) || ran != tc.ran || got != "" {
			t.Errorf("%q: ran=%t, stdout=%q, err=%v", tc.args, ran, got, err)
		}
	}
	t.Setenv("PROBE_FORMAT", "{{.Names}}")
	got, err := runWrite(t, app, out, &ran, "inspect", "--json=Status")
	if err != nil || strings.TrimSpace(got) != "{\n  \"Status\": \"Up\"\n}" {
		t.Errorf("explicit JSON overrides configured row format: stdout=%q, err=%v", got, err)
	}
}

func TestRowFormatRequiresOptIn(t *testing.T) {
	app, out, ran := writeApp(t)
	got, err := runWrite(t, app, out, ran, "list", "--format", "{{.name}}")
	if err == nil || clix.ExitCode(err) != 2 || *ran || got != "" {
		t.Fatalf("row template with default extension: ran=%t, stdout=%q, err=%v", *ran, got, err)
	}
}

func TestUsesRowTemplate(t *testing.T) {
	if format.UsesRowTemplate(nil) {
		t.Fatal("a nil app has no row formatter")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PROBE_FORMAT", "{{.key}}")
	app := clix.NewApp("probe", clix.WithAppOut(new(bytes.Buffer)), clix.WithAppErr(new(bytes.Buffer)))
	app.AddExtension(format.Extension{RowTemplates: true, Projections: true})
	var selected bool
	app.Root.AddCommand(clix.NewCommand("get", clix.WithCommandRun(func(ctx *clix.Context) error {
		selected = format.UsesRowTemplate(ctx.App)
		return nil
	})))
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{[]string{"get"}, true},
		{[]string{"get", "-f", "{{.other}}"}, true},
		{[]string{"get", "--format", "value(key)"}, false},
		{[]string{"get", "--json=key"}, false},
		{[]string{"get", "--template", "{{.key}}"}, false},
	} {
		if err := app.Run(t.Context(), tc.args); err != nil || selected != tc.want {
			t.Errorf("%q: selected=%v, err=%v; want %v", tc.args, selected, err, tc.want)
		}
	}
}
