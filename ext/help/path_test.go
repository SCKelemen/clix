package help

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// pathApp builds "probe widgets get" (alias "show") with a --watch flag.
func pathApp(t *testing.T) (*clix.App, *bytes.Buffer) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out := &bytes.Buffer{}
	app := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(&bytes.Buffer{}))
	app.AddExtension(Extension{})
	get := clix.NewCommand("get", clix.WithCommandRun(func(*clix.Context) error { return nil }))
	get.Short = "Get a widget"
	get.Aliases = []string{"show"}
	get.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "watch", Short: "w", Usage: "Watch for changes"}})
	list := clix.NewCommand("list", clix.WithCommandRun(func(*clix.Context) error { return nil }))
	list.Short = "List widgets"
	app.Root.AddCommand(clix.NewGroup("widgets", "Manage widgets", get, list))
	return app, out
}

// "help widgets get", the convention of git, go, kubectl and gh, must show
// the nested command's help, as must the quoted and --command spellings.
func TestHelpAcceptsNestedCommandPath(t *testing.T) {
	for _, args := range [][]string{
		{"help", "widgets", "get"},
		{"help", "widgets get"},
		{"help", "--command", "widgets get"},
		{"help", "--command", "widgets", "get"},
		{"help", "widgets", "show"},
	} {
		app, out := pathApp(t)
		if err := app.Run(context.Background(), args); err != nil {
			t.Errorf("%q: %v", args, err)
			continue
		}
		if !strings.Contains(out.String(), "--watch") {
			t.Errorf("%q did not show the help for widgets get:\n%s", args, out.String())
		}
	}
}

func TestHelpGroupPath(t *testing.T) {
	app, out := pathApp(t)
	if err := app.Run(context.Background(), []string{"help", "widgets"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Manage widgets") || strings.Contains(out.String(), "--watch") {
		t.Fatalf("help widgets should describe the group:\n%s", out.String())
	}
}

// A second run must not see the first run's path.
func TestHelpPathResetBetweenRuns(t *testing.T) {
	app, out := pathApp(t)
	ctx := context.Background()
	if err := app.Run(ctx, []string{"help", "widgets", "get"}); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := app.Run(ctx, []string{"help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "PROBE") {
		t.Fatalf("help after help widgets get should show the root help:\n%s", out.String())
	}
}

func TestHelpUnknownPath(t *testing.T) {
	for args, want := range map[string]string{
		"help nope":             `unknown command "nope" for "probe"`,
		"help widgets nope":     `unknown command "nope" for "probe widgets"`,
		"help widgets get more": `unknown command "more" for "probe widgets get"`,
	} {
		app, _ := pathApp(t)
		err := app.Run(context.Background(), strings.Fields(args))
		if err == nil || err.Error() != want {
			t.Errorf("%s: error = %v, want %q", args, err, want)
		}
	}
}

func TestHelpShowsVariadicArgument(t *testing.T) {
	app, out := pathApp(t)
	if err := app.Run(context.Background(), []string{"help", "--help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "<command>...") || !strings.Contains(out.String(), "probe help [command...]") {
		t.Fatalf("help --help should show the variadic command argument:\n%s", out.String())
	}
}

func TestHelpCompletesCommandNames(t *testing.T) {
	app, _ := pathApp(t)
	if err := app.ApplyExtensions(); err != nil {
		t.Fatal(err)
	}
	helpCmd := findChild(app.Root, "help")
	var flag *clix.Flag
	for _, f := range helpCmd.Flags.PositionalFlags() {
		if f.Name == "command" {
			flag = f
		}
	}
	complete := flag.Completer()
	if complete == nil {
		t.Fatal("help's command argument has no completion")
	}
	values := func(args []string, toComplete string) []string {
		var got []string
		for _, c := range complete(clix.CompletionRequest{App: app, Command: helpCmd, Flag: flag, Args: args, ToComplete: toComplete}) {
			got = append(got, c.Value)
		}
		return got
	}
	for _, tc := range []struct {
		args       []string
		toComplete string
		want       string
	}{
		{nil, "", "help widgets environment exit-codes"}, // then the help topics
		{[]string{"widgets"}, "", "get list"},
		{[]string{"widgets", "get"}, "", ""},
		{[]string{"nope"}, "", ""},
		{nil, "widgets ", "widgets get|widgets list"},
	} {
		sep := " "
		if strings.Contains(tc.toComplete, " ") {
			sep = "|"
		}
		if got := strings.Join(values(tc.args, tc.toComplete), sep); got != tc.want {
			t.Errorf("complete(%q, %q) = %q, want %q", tc.args, tc.toComplete, got, tc.want)
		}
	}
}
