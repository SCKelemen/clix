package clix_test

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
	"github.com/SCKelemen/clix/v2/ext/version"
)

// flagGroupApp is the tree the flag group tests run against:
//
//	myapp [--region/-r] [--quiet/-q] (Ambient) [--json/-j] [--yaml/-y] (Ambient)
//	      groups: --json | --yaml
//	└── labels
//	    └── update [--clear-labels/-c] [--remove-labels] [--user] [--password]
//	               [--file] [--url] [name] (positional)
//	               groups: --clear-labels | --remove-labels
//	                       --user & --password
//	                       one of --file, --url
type flagGroupApp struct {
	app    *clix.App
	out    *bytes.Buffer
	update *clix.Command
	ran    bool
	values map[string]string // flag -> value seen by the handler
	src    map[string]clix.Source
}

func newFlagGroupApp(t *testing.T, opts ...clix.AppOption) *flagGroupApp {
	t.Helper()
	g := &flagGroupApp{}
	g.app, g.out = newTestApp(t, "myapp", opts...)
	root := g.app.Flags()
	root.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "region", Short: "r"}})
	root.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "quiet", Short: "q", Ambient: true}})
	root.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "json", Short: "j", Ambient: true}})
	root.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "yaml", Short: "y", Ambient: true}})
	root.MarkMutuallyExclusive("json", "yaml")

	names := []string{"region", "json", "yaml", "clear-labels", "remove-labels", "user", "password", "file", "url", "name"}
	update := clix.NewCommand("update", clix.WithCommandShort("Update labels"), clix.WithCommandRun(func(ctx *clix.Context) error {
		g.ran = true
		g.values, g.src = map[string]string{}, map[string]clix.Source{}
		for _, n := range names {
			v, src, _ := ctx.EffectiveString(n)
			g.values[n], g.src[n] = v, src
		}
		return nil
	}))
	fs := update.Flags
	fs.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "clear-labels", Short: "c", Usage: "Remove all labels"}})
	fs.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "remove-labels", Usage: "Labels to remove"}})
	fs.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "user", Usage: "User name"}})
	fs.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "password", Usage: "Password", Sensitive: true}})
	fs.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "file", Usage: "Read labels from a file"}})
	fs.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "url", Usage: "Read labels from a URL"}})
	fs.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "name", Usage: "Resource", Positional: true}})
	fs.MarkMutuallyExclusive("clear-labels", "remove-labels")
	fs.MarkRequiredTogether("user", "password")
	fs.MarkOneRequired("file", "url")
	g.update = update
	g.app.Root.AddCommand(clix.NewGroup("labels", "Manage labels", update))
	if err := g.app.Validate(); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return g
}

func (g *flagGroupApp) run(args ...string) error {
	g.ran = false
	g.values, g.src = nil, nil
	return g.app.Run(bg, args)
}

// groupError checks that err is a *FlagGroupError of kind with message
// msg (any message when msg is "").
func groupError(t *testing.T, args []string, err error, kind clix.FlagGroupKind, msg string) {
	t.Helper()
	_ = checkGroupError(t, args, err, kind, msg)
}

// checkGroupError is groupError, returning the error.
func checkGroupError(t *testing.T, args []string, err error, kind clix.FlagGroupKind, msg string) *clix.FlagGroupError {
	t.Helper()
	var ge *clix.FlagGroupError
	if !errors.As(err, &ge) {
		t.Fatalf("%q: err = %v (%T), want a *FlagGroupError", args, err, err)
	}
	if ge.Kind != kind {
		t.Errorf("%q: Kind = %v, want %v", args, ge.Kind, kind)
	}
	if msg != "" && err.Error() != msg {
		t.Errorf("%q: message\n got  %q\n want %q", args, err.Error(), msg)
	}
	if code := clix.ExitCode(err); code != 2 {
		t.Errorf("%q: ExitCode = %d, want 2", args, code)
	}
	return ge
}

// Two flags of a mutually exclusive group on the command line are a usage
// error, however and wherever they are written.
func TestMutuallyExclusiveOnTheCommandLine(t *testing.T) {
	g := newFlagGroupApp(t)
	base := []string{"--file", "f"}
	conflicts := [][]string{
		{"labels", "update", "--clear-labels", "--remove-labels", "a"},
		{"labels", "update", "--remove-labels=a", "-c"},
		{"labels", "update", "--clear-labels=false", "--remove-labels", "a"},
		{"labels", "update", "-c", "--remove-labels", ""},
		{"--clear-labels", "labels", "update", "--remove-labels=a"},
	}
	for _, args := range conflicts {
		args = append(args, base...)
		err := g.run(args...)
		ge := checkGroupError(t, args, err, clix.FlagsMutuallyExclusive, "")
		if g.ran {
			t.Errorf("%q: the handler ran", args)
		}
		if !reflect.DeepEqual(ge.Flags, []string{"clear-labels", "remove-labels"}) || len(ge.Given) != 2 {
			t.Errorf("%q: Flags %q, Given %q", args, ge.Flags, ge.Given)
		}
		if !strings.Contains(err.Error(), "--clear-labels") || !strings.Contains(err.Error(), "--remove-labels") || !strings.HasSuffix(err.Error(), "cannot be used together") {
			t.Errorf("%q: message %q does not name both flags", args, err)
		}
	}

	// Root (Ambient) flags: in a cluster, and split around the path.
	for _, args := range [][]string{
		{"--json", "--yaml", "labels", "update", "--file", "f"},
		{"-jy", "labels", "update", "--file", "f"},
		{"labels", "update", "-qjy", "--file", "f"},
		{"--json", "labels", "update", "--file", "f", "--yaml"},
	} {
		groupError(t, args, g.run(args...), clix.FlagsMutuallyExclusive, "--json and --yaml cannot be used together")
	}

	// One flag of each group is fine.
	for _, args := range [][]string{
		{"labels", "update", "--clear-labels", "--json", "--file", "f"},
		{"--yaml", "labels", "update", "--remove-labels", "a", "--file", "f"},
		{"labels", "update", "--file", "f"},
	} {
		if err := g.run(args...); err != nil || !g.ran {
			t.Errorf("%q: %v (ran %v)", args, err, g.ran)
		}
	}
}

// A positional argument counts like the same flag given by name.
func TestMutuallyExclusivePositional(t *testing.T) {
	g := newFlagGroupApp(t)
	g.update.Flags.MarkMutuallyExclusive("name", "url")
	args := []string{"labels", "update", "--url", "u", "widget"}
	groupError(t, args, g.run(args...), clix.FlagsMutuallyExclusive, "--name and --url cannot be used together")
	if err := g.run("labels", "update", "--file", "f", "widget"); err != nil || g.values["name"] != "widget" {
		t.Fatalf("one positional: %v, name %q", err, g.values["name"])
	}
}

// A group member on the command line overrides the environment and the
// config file for the whole group, so the handler never sees two members
// set; without one, the environment beats the config file, as for a
// single flag.
func TestMutuallyExclusiveEnvAndConfig(t *testing.T) {
	g := newFlagGroupApp(t)
	writeConfigFile(t, g.app, "remove-labels: from-file\nyaml: true\n")
	t.Setenv("MYAPP_CLEAR_LABELS", "")

	// The command line beats the config file for the other member.
	if err := g.run("labels", "update", "--clear-labels", "--json", "--file", "f"); err != nil {
		t.Fatal(err)
	}
	if g.values["remove-labels"] != "" || g.src["remove-labels"] == clix.SourceConfigFile || g.values["yaml"] != "" {
		t.Fatalf("remove-labels %q (%v), yaml %q: a member was read from the config file", g.values["remove-labels"], g.src["remove-labels"], g.values["yaml"])
	}
	if g.values["clear-labels"] != "true" || g.values["json"] != "true" {
		t.Fatalf("clear-labels %q, json %q", g.values["clear-labels"], g.values["json"])
	}

	// Without a member on the command line, the config file applies.
	if err := g.run("labels", "update", "--file", "f"); err != nil {
		t.Fatal(err)
	}
	if g.values["remove-labels"] != "from-file" || g.src["remove-labels"] != clix.SourceConfigFile || g.values["yaml"] != "true" {
		t.Fatalf("remove-labels %q (%v), yaml %q", g.values["remove-labels"], g.src["remove-labels"], g.values["yaml"])
	}

	// The environment beats the config file for the whole group.
	t.Setenv("MYAPP_CLEAR_LABELS", "true")
	t.Setenv("MYAPP_JSON", "1")
	if err := g.run("labels", "update", "--file", "f"); err != nil {
		t.Fatal(err)
	}
	if g.values["clear-labels"] != "true" || g.src["clear-labels"] != clix.SourceEnvVar || g.values["remove-labels"] != "" {
		t.Fatalf("clear-labels %q (%v), remove-labels %q", g.values["clear-labels"], g.src["clear-labels"], g.values["remove-labels"])
	}
	if g.values["json"] != "true" || g.values["yaml"] != "" {
		t.Fatalf("json %q, yaml %q", g.values["json"], g.values["yaml"])
	}

	// And the command line beats the environment.
	if err := g.run("labels", "update", "--remove-labels", "cli", "--file", "f"); err != nil {
		t.Fatal(err)
	}
	if g.values["clear-labels"] != "" || g.values["remove-labels"] != "cli" {
		t.Fatalf("clear-labels %q, remove-labels %q", g.values["clear-labels"], g.values["remove-labels"])
	}

	// Two members from the environment are an error that names both
	// variables.
	t.Setenv("MYAPP_REMOVE_LABELS", "env")
	args := []string{"labels", "update", "--file", "f"}
	groupError(t, args, g.run(args...), clix.FlagsMutuallyExclusive,
		"--clear-labels (from $MYAPP_CLEAR_LABELS) and --remove-labels (from $MYAPP_REMOVE_LABELS) cannot be used together")
}

func TestMutuallyExclusiveConfigConflict(t *testing.T) {
	g := newFlagGroupApp(t)
	writeConfigFile(t, g.app, "json: true\nyaml: true\n")
	args := []string{"labels", "update", "--file", "f"}
	ge := checkGroupError(t, args, g.run(args...), clix.FlagsMutuallyExclusive,
		`--json (from config file key "json") and --yaml (from config file key "yaml") cannot be used together`)
	if !reflect.DeepEqual(ge.Sources, []clix.Source{clix.SourceConfigFile, clix.SourceConfigFile}) {
		t.Errorf("Sources = %v", ge.Sources)
	}
	// The command line settles it.
	if err := g.run("--yaml", "labels", "update", "--file", "f"); err != nil || g.values["json"] != "" || g.values["yaml"] != "true" {
		t.Fatalf("--yaml: %v, json %q, yaml %q", err, g.values["json"], g.values["yaml"])
	}
}

// A command marked AllowConfigErrors ignores both members with a warning,
// as it ignores a value a flag rejects.
func TestMutuallyExclusiveConflictTolerated(t *testing.T) {
	g := newFlagGroupApp(t)
	writeConfigFile(t, g.app, "json: true\nyaml: true\n")
	clix.AllowConfigErrors(g.update)
	if err := g.run("labels", "update", "--file", "f"); err != nil {
		t.Fatal(err)
	}
	if g.values["json"] != "" || g.values["yaml"] != "" {
		t.Fatalf("json %q, yaml %q; want both defaults", g.values["json"], g.values["yaml"])
	}
	if !strings.Contains(g.out.String(), "myapp: ignoring --json (from config file key \"json\") and --yaml") {
		t.Fatalf("no warning: %q", g.out.String())
	}
}

// Required together: a member set from any source but its default needs
// the others, from any source.
func TestRequiredTogether(t *testing.T) {
	g := newFlagGroupApp(t)
	args := []string{"labels", "update", "--file", "f", "--user", "u"}
	ge := checkGroupError(t, args, g.run(args...), clix.FlagsRequiredTogether,
		"--user and --password must be used together: missing --password (or $MYAPP_PASSWORD)")
	if !reflect.DeepEqual(ge.Given, []string{"user"}) || !reflect.DeepEqual(ge.Missing, []string{"password"}) {
		t.Errorf("Given %q, Missing %q", ge.Given, ge.Missing)
	}
	if err := g.run("labels", "update", "--file", "f", "--user", "u", "--password", "p"); err != nil || !g.ran {
		t.Fatalf("both: %v", err)
	}
	if err := g.run("labels", "update", "--file", "f"); err != nil || !g.ran {
		t.Fatalf("neither: %v", err)
	}

	// The environment and the config file count.
	g = newFlagGroupApp(t)
	writeConfigFile(t, g.app, "password: from-file\n")
	if err := g.run(args...); err != nil || g.values["password"] != "from-file" {
		t.Fatalf("password from the config file: %v, %q", err, g.values["password"])
	}
	t.Setenv("MYAPP_USER", "env-user")
	if err := g.run("labels", "update", "--file", "f"); err != nil || g.values["user"] != "env-user" {
		t.Fatalf("user from the environment: %v, %q", err, g.values["user"])
	}
	g = newFlagGroupApp(t)
	args = []string{"labels", "update", "--file", "f"}
	groupError(t, args, g.run(args...), clix.FlagsRequiredTogether, "--user and --password must be used together: missing --password (or $MYAPP_PASSWORD)")
}

// A default never counts as given.
func TestRequiredTogetherDefault(t *testing.T) {
	app, _ := newTestApp(t, "myapp")
	cmd := clix.NewCommand("login", clix.WithCommandRun(noop))
	cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "user"}, Default: "admin"})
	cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "password"}})
	cmd.Flags.MarkRequiredTogether("user", "password")
	app.Root.AddCommand(cmd)
	if err := app.Run(bg, []string{"login"}); err != nil {
		t.Fatalf("default only: %v", err)
	}
	args := []string{"login", "--user", "root"}
	groupError(t, args, app.Run(bg, args), clix.FlagsRequiredTogether, "--user and --password must be used together: missing --password (or $MYAPP_PASSWORD)")
}

func TestOneRequired(t *testing.T) {
	g := newFlagGroupApp(t)
	args := []string{"labels", "update"}
	ge := checkGroupError(t, args, g.run(args...), clix.FlagsOneRequired, "one of --file or --url is required")
	if !reflect.DeepEqual(ge.Missing, []string{"file", "url"}) || g.ran {
		t.Errorf("Missing %q, ran %v", ge.Missing, g.ran)
	}
	for _, args := range [][]string{
		{"labels", "update", "--file", "f"},
		{"labels", "update", "--url", "u"},
		{"labels", "update", "--url", "u", "--file", "f"},
	} {
		if err := g.run(args...); err != nil || !g.ran {
			t.Errorf("%q: %v", args, err)
		}
	}
	t.Setenv("MYAPP_URL", "https://example.com")
	if err := g.run("labels", "update"); err != nil || g.values["url"] != "https://example.com" {
		t.Fatalf("url from the environment: %v", err)
	}
	t.Setenv("MYAPP_URL", "")
	g = newFlagGroupApp(t)
	writeConfigFile(t, g.app, "file: labels.txt\n")
	if err := g.run("labels", "update"); err != nil || g.values["file"] != "labels.txt" {
		t.Fatalf("file from the config file: %v", err)
	}
}

// Exactly one: mutually exclusive and one required over the same flags.
func TestExactlyOne(t *testing.T) {
	app, out := newTestApp(t, "myapp")
	cmd := clix.NewCommand("import", clix.WithCommandRun(noop))
	cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "file"}})
	cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "url"}})
	cmd.Flags.MarkMutuallyExclusive("file", "url")
	cmd.Flags.MarkOneRequired("url", "file")
	app.Root.AddCommand(cmd)
	assertValidate(t, app)
	groupError(t, nil, app.Run(bg, []string{"import"}), clix.FlagsOneRequired, "one of --url or --file is required")
	groupError(t, nil, app.Run(bg, []string{"import", "--file", "a", "--url", "b"}), clix.FlagsMutuallyExclusive, "--file and --url cannot be used together")
	if err := app.Run(bg, []string{"import", "--url", "b"}); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := app.Run(bg, []string{"import", "--help"}); err != nil {
		t.Fatal(err)
	}
	help := out.String()
	for _, want := range []string{"myapp import [flags] (--file=FILE | --url=URL)\n", "  Exactly one of these must be given: --file, --url\n"} {
		if !strings.Contains(help, want) {
			t.Errorf("help lacks %q:\n%s", want, help)
		}
	}
	if strings.Count(help, "--file=FILE") != 1 || strings.Contains(help, "At least one") || strings.Contains(help, "At most one") {
		t.Errorf("the two groups are not shown as one:\n%s", help)
	}
}

// A group may name flags of the commands above it, resolved as the parser
// resolves a flag written after the command name.
func TestFlagGroupsNameAncestorFlags(t *testing.T) {
	g := newFlagGroupApp(t)
	g.update.Flags.MarkMutuallyExclusive("region", "url")
	args := []string{"--region", "eu", "labels", "update", "--url", "u"}
	groupError(t, args, g.run(args...), clix.FlagsMutuallyExclusive, "--region and --url cannot be used together")
	assertValidate(t, g.app)

	// The group belongs to update: other commands do not check it.
	other := clix.NewCommand("other", clix.WithCommandRun(noop))
	other.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "url"}})
	g.app.Root.AddCommand(other)
	if err := g.run("--region", "eu", "other", "--url", "u"); err != nil {
		t.Fatal(err)
	}
}

// The root's groups apply to every command, except those added by an
// extension (and their subcommands), which skip them like required flags.
func TestRootGroupsAndExtensionCommands(t *testing.T) {
	g := newFlagGroupApp(t)
	g.app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project"}})
	g.app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project-file"}})
	g.app.Flags().MarkOneRequired("project", "project-file")
	g.app.AddExtension(version.Extension{Version: "1.0.0"})
	args := []string{"labels", "update", "--file", "f"}
	groupError(t, args, g.run(args...), clix.FlagsOneRequired, "one of --project or --project-file is required")
	g.app.ConfigPath = ""
	t.Setenv("MYAPP_JSON", "1")
	t.Setenv("MYAPP_YAML", "1")
	for _, args := range [][]string{{"version"}, {"--json", "--yaml", "version"}} {
		if err := g.run(args...); err != nil {
			t.Errorf("%q: %v", args, err)
		}
	}
}

// Groups work with flags an extension owns, and resolve a name to the
// flag the parser uses when the app and an extension both define it.
func TestFlagGroupsAndExtensionFlags(t *testing.T) {
	for _, order := range registrationOrders {
		t.Run(order, func(t *testing.T) {
			app, _ := newTestApp(t, "myapp")
			var csv bool
			withOrder(t, app, order, []clix.Extension{format.Extension{}}, func() {
				app.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "csv"}, Value: &csv})
				app.Flags().MarkMutuallyExclusive("format", "csv")
				app.Root.AddCommand(clix.NewCommand("ls", clix.WithCommandRun(noop)))
			})
			assertValidate(t, app)
			args := []string{"--format", "json", "ls", "--csv"}
			groupError(t, args, app.Run(bg, args), clix.FlagsMutuallyExclusive, "--format and --csv cannot be used together")
			if err := app.Run(bg, []string{"ls", "-f", "yaml"}); err != nil || format.OutputFormat(app) != "yaml" {
				t.Fatalf("ls -f yaml: %v", err)
			}
			// --csv on the command line keeps --format from the
			// environment: the extension's flag keeps its default.
			t.Setenv("MYAPP_FORMAT", "json")
			if err := app.Run(bg, []string{"ls", "--csv"}); err != nil || format.OutputFormat(app) != "text" || !csv {
				t.Fatalf("ls --csv with MYAPP_FORMAT: %v, format %q", err, format.OutputFormat(app))
			}
		})
	}

	// An extension's flag that the root's flag of the same name shadows on
	// a command: the group means the root's flag, which the parser sets.
	app, _ := newTestApp(t, "myapp")
	var project, extProject string
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project"}, Value: &project})
	deploy := clix.NewCommand("deploy", clix.WithCommandRun(noop))
	deploy.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "region"}})
	deploy.Flags.MarkMutuallyExclusive("project", "region")
	app.Root.AddCommand(deploy)
	app.AddExtension(extensionFunc(func(a *clix.App) error {
		deploy.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project"}, Value: &extProject})
		return nil
	}))
	args := []string{"deploy", "--project", "p", "--region", "r"}
	groupError(t, args, app.Run(bg, args), clix.FlagsMutuallyExclusive, "--project and --region cannot be used together")
	if project != "p" || extProject != "" {
		t.Fatalf("root project %q, extension's %q", project, extProject)
	}

	// A group an extension registers on its own command works too.
	app, _ = newTestApp(t, "myapp")
	app.AddExtension(extensionFunc(func(a *clix.App) error {
		cmd := clix.NewCommand("sync", clix.WithCommandRun(noop))
		cmd.IsExtensionCommand = true
		cmd.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "push"}})
		cmd.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "pull"}})
		cmd.Flags.MarkMutuallyExclusive("push", "pull")
		a.Root.AddCommand(cmd)
		return nil
	}))
	groupError(t, nil, app.Run(bg, []string{"sync", "--push", "--pull"}), clix.FlagsMutuallyExclusive, "--push and --pull cannot be used together")
	assertValidate(t, app)
}

// Help shows the groups in the usage line, as gcloud does, and says what
// each one means under the flags.
func TestFlagGroupsHelp(t *testing.T) {
	g := newFlagGroupApp(t)
	g.out.Reset()
	if err := g.run("labels", "update", "--help"); err != nil {
		t.Fatal(err)
	}
	help := g.out.String()
	wants := []string{
		"USAGE\n  myapp labels update [flags] [--clear-labels | --remove-labels=REMOVE_LABELS] [--user=USER --password=PASSWORD] (--file=FILE | --url=URL) [name]\n",
		"\n\n  At most one of these can be given: --clear-labels, --remove-labels\n" +
			"  All or none of these must be given: --user, --password\n" +
			"  At least one of these must be given: --file, --url\n\nGLOBAL FLAGS",
	}
	for _, want := range wants {
		if !strings.Contains(help, want) {
			t.Errorf("help lacks %q:\n%s", want, help)
		}
	}
	g.out.Reset()
	if err := g.run("--help"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(g.out.String(), "myapp [flags] [--json | --yaml]\n") {
		t.Errorf("root help lacks the root's group:\n%s", g.out.String())
	}
	// --help wins over a broken group, and a group of commands only shows
	// help.
	for _, args := range [][]string{{"labels", "update", "-c", "--remove-labels", "x", "--help"}, {"labels"}, {"--json", "--yaml", "labels"}} {
		if err := g.run(args...); err != nil || g.ran {
			t.Errorf("%q: %v (ran %v)", args, err, g.ran)
		}
	}
	// A custom Usage is left alone.
	g.update.Usage = "myapp labels update NAME"
	g.out.Reset()
	if err := g.run("labels", "update", "-h"); err != nil || !strings.Contains(g.out.String(), "  myapp labels update NAME\n") {
		t.Errorf("custom usage: %v\n%s", err, g.out.String())
	}
}

func TestFlagGroupsValidate(t *testing.T) {
	app, _ := newTestApp(t, "myapp")
	cmd := clix.NewCommand("cmd", clix.WithCommandRun(noop))
	cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "a"}})
	cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "b", Required: true}})
	cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "c", Required: true}, Default: "x"})
	cmd.Flags.MarkMutuallyExclusive("a", "nope")
	cmd.Flags.MarkRequiredTogether("a")
	cmd.Flags.MarkOneRequired("a", "a", "--a")
	cmd.Flags.MarkMutuallyExclusive("a", "b")
	cmd.Flags.MarkMutuallyExclusive("a", "c")      // c has a default: fine
	cmd.Flags.MarkMutuallyExclusive("a", "format") // from ext/format, added below
	cmd.Flags.MarkOneRequired("a", "help")
	app.Root.AddCommand(cmd)
	assertValidate(t, app,
		"myapp cmd: mutually exclusive flags --a, --nope: --nope is not a flag of this command or of a command above it",
		"myapp cmd: required together flags --a: a group needs at least two flags",
		"myapp cmd: one required flags --a: a group needs at least two flags",
		"myapp cmd: mutually exclusive flags --a, --b: --b is required, so the other flags of the group can never be given",
		"myapp cmd: mutually exclusive flags --a, --format: --format is not a flag of this command or of a command above it",
		"myapp cmd: one required flags --a, --help: --help is not a flag of this command or of a command above it",
	)
	app.AddExtension(format.Extension{})
	if err := app.Validate(); err == nil || strings.Contains(err.Error(), "--format is not a flag") {
		t.Fatalf("with ext/format: %v", err)
	}
	if got := cmd.Flags.FlagGroups(); len(got) != 7 || got[2].Kind != clix.FlagsOneRequired || !reflect.DeepEqual(got[2].Flags, []string{"a"}) {
		t.Fatalf("FlagGroups = %+v", got)
	}
	// FlagGroups returns copies.
	cmd.Flags.FlagGroups()[0].Flags[0] = "changed"
	if cmd.Flags.FlagGroups()[0].Flags[0] != "a" {
		t.Fatal("FlagGroups shares its slices")
	}
}

// Groups registered on app.Flags() before Root is replaced move with the
// flags.
func TestFlagGroupsMoveWithReplacedRoot(t *testing.T) {
	app, _ := newTestApp(t, "myapp")
	app.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "json"}})
	app.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "yaml"}})
	app.Flags().MarkMutuallyExclusive("json", "yaml")
	app.Root = clix.NewCommand("myapp", clix.WithCommandRun(noop))
	groupError(t, nil, app.Run(bg, []string{"--json", "--yaml"}), clix.FlagsMutuallyExclusive, "--json and --yaml cannot be used together")
	if len(app.Root.Flags.FlagGroups()) != 1 {
		t.Fatalf("groups = %+v", app.Root.Flags.FlagGroups())
	}
}

// Ambient members do not stop prompting, and a requirement is checked
// before any prompt, counting the flags about to be prompted for.
func TestFlagGroupsAndPrompting(t *testing.T) {
	g := newFlagGroupApp(t, clix.WithAppInteractive(clix.InteractiveAlways))
	g.update.Flags.Lookup("name").Required = true
	p := &askPrompter{answers: map[string]string{"Name": "widget", "Url": "https://example.com"}}
	g.app.Prompter = p

	// Only Ambient flags: the missing required flag is prompted for, but
	// the one-required group is broken, so nothing is asked.
	args := []string{"--json", "labels", "update"}
	groupError(t, args, g.run(args...), clix.FlagsOneRequired, "one of --file or --url is required")
	if len(p.asked) != 0 {
		t.Fatalf("asked %q before reporting the group", p.asked)
	}

	// A member that is required and missing is prompted for and satisfies
	// the group.
	g.update.Flags.Lookup("url").Required = true
	if err := g.run("--json", "labels", "update"); err != nil || g.values["url"] != "https://example.com" || g.values["name"] != "widget" {
		t.Fatalf("%v: url %q, name %q", err, g.values["url"], g.values["name"])
	}
	if !reflect.DeepEqual(p.asked, []string{"Url", "Name"}) {
		t.Fatalf("asked %q", p.asked)
	}

	// Two Ambient members are still a conflict.
	args = []string{"--json", "--yaml", "labels", "update"}
	groupError(t, args, g.run(args...), clix.FlagsMutuallyExclusive, "--json and --yaml cannot be used together")
}

// Nothing a Run decides about a group leaks into the next Run.
func TestFlagGroupsRepeatedRuns(t *testing.T) {
	g := newFlagGroupApp(t)
	writeConfigFile(t, g.app, "remove-labels: from-file\n")
	for i := 0; i < 2; i++ {
		if err := g.run("labels", "update", "-c", "--file", "f"); err != nil || g.values["remove-labels"] != "" {
			t.Fatalf("run %d with -c: %v, remove-labels %q", i, err, g.values["remove-labels"])
		}
		if err := g.run("labels", "update", "--file", "f"); err != nil || g.values["remove-labels"] != "from-file" || g.values["clear-labels"] != "" {
			t.Fatalf("run %d without: %v, %v", i, err, g.values)
		}
		groupError(t, nil, g.run("labels", "update", "-c", "--remove-labels", "x", "--file", "f"), clix.FlagsMutuallyExclusive, "")
	}
}

// A Run that a handler starts keeps what the outer Run decided about its
// groups: after it returns, a member the outer command line ruled out is
// still not read from the config file.
func TestFlagGroupsNestedRun(t *testing.T) {
	app, _ := newTestApp(t, "myapp")
	writeConfigFile(t, app, "y: from-file\n")
	var seen string
	outer := clix.NewCommand("outer")
	outer.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "x"}})
	outer.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "y"}})
	outer.Flags.MarkMutuallyExclusive("x", "y")
	outer.Run = func(ctx *clix.Context) error {
		if err := ctx.App.Run(ctx, []string{"inner"}); err != nil {
			return err
		}
		seen, _ = ctx.String("y")
		return nil
	}
	app.Root.AddCommand(outer)
	app.Root.AddCommand(clix.NewCommand("inner", clix.WithCommandRun(noop)))
	if err := app.Run(bg, []string{"outer", "--x"}); err != nil || seen != "" {
		t.Fatalf("outer --x: %v, y %q", err, seen)
	}
	if err := app.Run(bg, []string{"outer"}); err != nil || seen != "from-file" {
		t.Fatalf("outer: %v, y %q", err, seen)
	}
}
