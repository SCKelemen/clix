package format_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
)

// The list flags apply before a gcloud format: filter, then sort, then
// limit, then the projection.
func TestListFlagsWithProjections(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PROBE_FORMAT", "")
	t.Setenv("LC_ALL", "C")
	out := &bytes.Buffer{}
	app := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(&bytes.Buffer{}), clix.WithAppInteractive(clix.InteractiveNever))
	app.AddExtension(format.Extension{Projections: true})
	app.Root.AddCommand(clix.NewCommand("list", format.WithListFlags(), clix.WithCommandRun(func(ctx *clix.Context) error {
		return format.Write(ctx, projects)
	})))
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--filter", "labels.env=prod", "--sort-by", "projectId", "--format", "table(projectId:label=ID, name)"},
			"ID        NAME\napi       API\nweb-prod  Web\n"},
		{[]string{"--sort-by", "~createTime", "--limit", "2", "--format", "value(projectId, createTime)"},
			"web-prod\t2021-02-10T19:19:49Z\napi\t2019-06-01T10:00:00Z\n"},
		{[]string{"--filter", "lifecycleState!=ACTIVE", "--format", "csv(projectId, labels.env)"},
			"project_id,env\nold,\n"},
		{[]string{"--filter", "name:nope", "--format", "table(name)"}, ""},
		{[]string{"--limit", "1", "--format", "json(projectId)"}, "[\n  {\n    \"projectId\": \"web-prod\"\n  }\n]\n"},
		{[]string{"--limit", "1", "--format", "flattened(name, labels)"}, "---\nlabels.env: prod\nname:       Web\n"},
	} {
		out.Reset()
		if err := app.Run(context.Background(), append([]string{"list"}, tc.args...)); err != nil {
			t.Errorf("%q: %v", tc.args, err)
			continue
		}
		if out.String() != tc.want {
			t.Errorf("%q:\ngot  %q\nwant %q", tc.args, out.String(), tc.want)
		}
	}
	// A projection on the command line still conflicts with --json, with
	// the list flags too.
	err := app.Run(context.Background(), []string{"list", "--limit", "1", "--format", "table(name)", "--json", "name"})
	if err == nil || clix.ExitCode(err) != 2 || !strings.Contains(err.Error(), "--json") {
		t.Errorf("--format table with --json: %v", err)
	}
}
