package clix_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/version"
)

// aliasApp is a gh-like tree with aliases enabled:
//
//	myapp [--repo/-R]
//	├── pr
//	│   ├── checkout [--force] <number>
//	│   └── create --title --body
//	├── issue
//	│   └── list [--label]... [--web]
//	├── deploy (alias ship)
//	└── secret                        Hidden
type aliasApp struct {
	app *clix.App
	ran string
	got map[string]string
	pos []string
}

func newAliasApp(t *testing.T, config string) *aliasApp {
	t.Helper()
	a := &aliasApp{}
	a.app, _ = newTestApp(t, "myapp")
	writeConfigFile(t, a.app, config)
	a.app.ExpandAliases = true
	a.app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "repo", Short: "R"}})
	run := func(flags ...string) clix.Handler {
		return func(ctx *clix.Context) error {
			a.ran = ctx.Command.Path()
			a.got = map[string]string{}
			for _, f := range flags {
				if v, ok := ctx.String(f); ok {
					a.got[f] = v
				}
			}
			return nil
		}
	}
	checkout := clix.NewCommand("checkout", clix.WithCommandRun(run("number", "force", "repo")))
	checkout.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "force"}})
	checkout.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "number", Positional: true}})
	create := clix.NewCommand("create", clix.WithCommandRun(run("title", "body")))
	create.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "title"}})
	create.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "body"}})
	list := clix.NewCommand("list", clix.WithCommandRun(func(ctx *clix.Context) error {
		a.ran = ctx.Command.Path()
		a.pos, _ = ctx.Command.Flags.StringSlice("label")
		a.got = map[string]string{}
		if v, ok := ctx.String("web"); ok {
			a.got["web"] = v
		}
		return nil
	}))
	list.Flags.StringSliceVar(clix.StringSliceVarOptions{FlagOptions: clix.FlagOptions{Name: "label"}})
	list.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "web"}})
	a.app.Root.AddCommand(clix.NewGroup("pr", "Pull requests", checkout, create))
	a.app.Root.AddCommand(clix.NewGroup("issue", "Issues", list))
	a.app.Root.AddCommand(clix.NewCommand("deploy", clix.WithCommandAliases("ship"), clix.WithCommandRun(run())))
	a.app.Root.AddCommand(clix.NewCommand("secret", clix.WithCommandHidden(true), clix.WithCommandRun(run())))
	return a
}

func (a *aliasApp) run(args ...string) error {
	a.ran, a.got, a.pos = "", nil, nil
	return a.app.Run(bg, args)
}

func TestAliasExpansion(t *testing.T) {
	a := newAliasApp(t, `aliases:
  co: pr checkout
  il: issue list --label=$1
  two: pr create --title "$1" --body=$2
  sq: pr create --title 'literal $1 $HOME' --body "a \"quoted\" \$1"
  lit: pr create --title $HOME --body $(touch pwned)
`)
	for _, tc := range []struct {
		args []string
		want []string
	}{
		{[]string{"co", "12"}, []string{"pr", "checkout", "12"}},
		{[]string{"co", "12", "--force", "-R", "x/y"}, []string{"pr", "checkout", "12", "--force", "-R", "x/y"}},
		{[]string{"co"}, []string{"pr", "checkout"}},
		{[]string{"il", "bug", "--web"}, []string{"issue", "list", "--label=bug", "--web"}},
		// Arguments are whole words, never split or read again.
		{[]string{"il", "a b,c", "--web"}, []string{"issue", "list", "--label=a b,c", "--web"}},
		{[]string{"co", "1 --force"}, []string{"pr", "checkout", "1 --force"}},
		{[]string{"two", "Fix it", "--force"}, []string{"pr", "create", "--title", "Fix it", "--body=--force"}},
		{[]string{"two", "t", "b", "extra"}, []string{"pr", "create", "--title", "t", "--body=b", "extra"}},
		{[]string{"sq", "arg"}, []string{"pr", "create", "--title", "literal $1 $HOME", "--body", `a "quoted" $1`, "arg"}},
		{[]string{"lit"}, []string{"pr", "create", "--title", "$HOME", "--body", "$(touch", "pwned)"}},
	} {
		got, isAlias, err := a.app.ExpandAlias(tc.args)
		if err != nil || !isAlias || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ExpandAlias(%q) = %q, %v, %v; want %q", tc.args, got, isAlias, err, tc.want)
		}
	}

	// gh's rule: arguments up to the highest $N are used for the
	// placeholders (one not referenced is dropped), the rest appended.
	a.app.Config.Set("aliases.skip", "issue list --label=$2")
	if got, _, _ := a.app.ExpandAlias([]string{"skip", "A", "B", "C"}); !reflect.DeepEqual(got, []string{"issue", "list", "--label=B", "C"}) {
		t.Errorf("skip A B C = %q", got)
	}

	// Run routes the expansion like any command line.
	if err := a.run("co", "42", "--force"); err != nil || a.ran != "myapp pr checkout" || a.got["number"] != "42" || a.got["force"] != "true" {
		t.Fatalf("co 42 --force: %v, ran %q, %v", err, a.ran, a.got)
	}
	if err := a.run("il", "bug", "--web"); err != nil || a.ran != "myapp issue list" || !reflect.DeepEqual(a.pos, []string{"bug"}) || a.got["web"] != "true" {
		t.Fatalf("il bug --web: %v, ran %q, labels %q", err, a.ran, a.pos)
	}
	if err := a.run("two", "Fix it", "body text"); err != nil || a.got["title"] != "Fix it" || a.got["body"] != "body text" {
		t.Fatalf("two: %v, %v", err, a.got)
	}
	// --help after an alias shows the expansion's help.
	if err := a.run("co", "--help"); err != nil || a.ran != "" {
		t.Fatalf("co --help: %v, ran %q", err, a.ran)
	}
}

func TestAliasExpansionErrors(t *testing.T) {
	a := newAliasApp(t, `aliases:
  two: pr create --title $1 --body $2
  shell: "!echo pwned"
  nowhere: nosuch command
  flagfirst: --repo x pr list
  open: pr create --title "unterminated
  empty: "  "
`)
	for args, want := range map[string]string{
		"two x":     `alias "two" needs 2 arguments ($1 to $2 in "pr create --title $1 --body $2"), got 1`,
		"shell":     `alias "shell": shell aliases ("!...") are not supported`,
		"nowhere":   `alias "nowhere" expands to "nosuch command", which is not a command of myapp`,
		"flagfirst": `alias "flagfirst" expands to "--repo x pr list", which is not a command of myapp`,
		"open":      `alias "open": unterminated "`,
		"empty":     `alias "empty": the expansion is empty`,
	} {
		err := a.run(strings.Fields(args)...)
		if err == nil || !strings.Contains(err.Error(), want) || clix.ExitCode(err) != 2 || a.ran != "" {
			t.Errorf("%s: %v (exit %d, ran %q), want %q", args, err, clix.ExitCode(err), a.ran, want)
		}
	}
}

// Aliases never shadow commands: a command, a command's alias, a hidden
// command or an extension's command with the alias's name runs instead.
func TestAliasesNeverShadowCommands(t *testing.T) {
	a := newAliasApp(t, `aliases:
  deploy: pr create
  ship: pr create
  secret: pr create
  version: pr create
  PR: issue list
`)
	a.app.AddExtension(version.Extension{Version: "1.0.0"})
	for _, args := range [][]string{{"deploy"}, {"ship"}, {"secret"}} {
		if err := a.run(args...); err != nil || a.ran != "myapp "+map[string]string{"deploy": "deploy", "ship": "deploy", "secret": "secret"}[args[0]] {
			t.Errorf("%q: %v, ran %q", args, err, a.ran)
		}
	}
	if err := a.run("version"); err != nil || a.ran != "" {
		t.Errorf("version: %v, ran %q", err, a.ran)
	}
	// "PR" routes to the pr group (ASCII case is ignored), not the alias.
	if err := a.run("PR", "checkout", "1"); err != nil || a.ran != "myapp pr checkout" {
		t.Errorf("PR checkout 1: %v, ran %q", err, a.ran)
	}
	for _, name := range []string{"deploy", "ship", "secret", "version", "PR", "Deploy"} {
		err := a.app.CheckAlias(name, "pr create")
		if err == nil || !strings.Contains(err.Error(), "would shadow the command") {
			t.Errorf("CheckAlias(%q): %v", name, err)
		}
	}
	// With CaseSensitive, "PR" is not the pr group.
	a.app.CaseSensitive = true
	if err := a.app.CheckAlias("PR", "issue list"); err != nil {
		t.Errorf("CheckAlias(PR) when case-sensitive: %v", err)
	}
	if err := a.run("PR"); err != nil || a.ran != "myapp issue list" {
		t.Errorf("PR when case-sensitive: %v, ran %q", err, a.ran)
	}
}

// An alias is only the first argument, and only where the root would
// reject the word.
func TestAliasOnlyFirstWord(t *testing.T) {
	a := newAliasApp(t, "aliases:\n  co: pr checkout\n  Co2: pr checkout\n")
	for _, args := range [][]string{{"--repo", "x", "co"}, {"pr", "co"}, {"--", "co"}} {
		if err := a.run(args...); err == nil || a.ran != "" {
			t.Errorf("%q: %v, ran %q; want an error", args, err, a.ran)
		}
	}
	// Names match like commands: ASCII case is ignored by default.
	if err := a.run("CO", "7"); err != nil || a.ran != "myapp pr checkout" || a.got["number"] != "7" {
		t.Errorf("CO 7: %v, ran %q", err, a.ran)
	}
	if err := a.run("co2"); err != nil || a.ran != "myapp pr checkout" {
		t.Errorf("co2: %v, ran %q", err, a.ran)
	}
	// Off: no expansion.
	a.app.ExpandAliases = false
	if err := a.run("co", "1"); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("with ExpandAliases off: %v", err)
	}
	a.app.ExpandAliases = true

	// A root that takes positional arguments keeps its word.
	app, _ := newTestApp(t, "echo")
	writeConfigFile(t, app, "aliases:\n  hi: say\n")
	app.ExpandAliases = true
	var text string
	app.Root.Run = func(*clix.Context) error { return nil }
	app.Root.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "text", Positional: true}, Value: &text})
	app.Root.AddCommand(clix.NewCommand("say", clix.WithCommandRun(noop)))
	if err := app.Run(bg, []string{"hi"}); err != nil || text != "hi" {
		t.Errorf("echo hi: %v, text %q", err, text)
	}
}

func TestCheckAlias(t *testing.T) {
	a := newAliasApp(t, "")
	for _, tc := range []struct{ name, expansion, want string }{
		{"co", "pr checkout", ""},
		{"co-2_x", "pr checkout $1", ""},
		{"il", `issue list --label="$1"`, ""},
		{"", "pr", "invalid alias name"},
		{"a.b", "pr", "invalid alias name"},
		{"-x", "pr", "invalid alias name"},
		{"a b", "pr", "invalid alias name"},
		{"_x", "pr", "invalid alias name"},
		{"x", "!rm -rf /", "shell aliases"},
		{"x", "  !ls", "shell aliases"},
		{"x", "", "the expansion is empty"},
		{"x", "nosuch", "must start with a command"},
		{"x", "$1 list", "must start with a command"},
		{"x", "--repo a pr", "must start with a command"},
		{"x", `pr "open`, "unterminated"},
		{"x", "pr 'open", "unterminated"},
	} {
		err := a.app.CheckAlias(tc.name, tc.expansion)
		if tc.want == "" && err != nil || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
			t.Errorf("CheckAlias(%q, %q) = %v, want %q", tc.name, tc.expansion, err, tc.want)
		}
	}
	if err := a.app.SetAlias("co", "pr checkout"); err != nil {
		t.Fatal(err)
	}
	if got := a.app.Aliases(); !reflect.DeepEqual(got, map[string]string{"co": "pr checkout"}) {
		t.Fatalf("Aliases() = %v", got)
	}
	a.app.Config.Set("aliases.bad.name", "pr")
	if got := a.app.AliasNames(); !reflect.DeepEqual(got, []string{"co"}) {
		t.Fatalf("AliasNames() = %q", got)
	}
	if !a.app.DeleteAlias("co") || a.app.DeleteAlias("co") {
		t.Fatal("DeleteAlias")
	}
}

// A first word that may be an alias needs the config file: when it does
// not load, that is the error.
func TestAliasBrokenConfig(t *testing.T) {
	a := newAliasApp(t, "aliases: [unclosed\n")
	err := a.run("co", "1")
	if err == nil || !strings.Contains(err.Error(), "failed to parse config file") {
		t.Fatalf("co with a broken config: %v", err)
	}
	if err := a.run("--help"); err != nil {
		t.Fatalf("--help: %v", err)
	}
}

// Root help lists the aliases under ALIAS COMMANDS, except shadowed ones.
func TestAliasHelp(t *testing.T) {
	a := newAliasApp(t, "aliases:\n  co: pr checkout\n  il: issue list --label=$1\n  deploy: pr create\n")
	out := &strings.Builder{}
	if err := (clix.HelpRenderer{App: a.app, Command: a.app.Root}).Render(out); err != nil {
		t.Fatal(err)
	}
	if err := a.run("--help"); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := (clix.HelpRenderer{App: a.app, Command: a.app.Root}).Render(out); err != nil {
		t.Fatal(err)
	}
	want := "ALIAS COMMANDS\n  co                   Alias for \"pr checkout\"\n  il                   Alias for \"issue list --label=$1\"\n\n"
	if !strings.Contains(out.String(), want) || strings.Contains(out.String(), `Alias for "pr create"`) {
		t.Fatalf("help:\n%s", out.String())
	}
	a.app.ExpandAliases = false
	out.Reset()
	_ = (clix.HelpRenderer{App: a.app, Command: a.app.Root}).Render(out)
	if strings.Contains(out.String(), "ALIAS COMMANDS") {
		t.Fatalf("aliases shown while off:\n%s", out.String())
	}
}

// Nothing an expansion does leaks into the next Run.
func TestAliasRepeatedRuns(t *testing.T) {
	a := newAliasApp(t, "aliases:\n  fco: pr checkout --force\n")
	for i := 0; i < 2; i++ {
		if err := a.run("fco", "1"); err != nil || a.got["force"] != "true" {
			t.Fatalf("run %d: fco 1: %v, %v", i, err, a.got)
		}
		if err := a.run("pr", "checkout", "2"); err != nil || a.got["force"] == "true" {
			t.Fatalf("run %d: pr checkout 2: %v, %v", i, err, a.got)
		}
	}
}
