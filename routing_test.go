package clix_test

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// routingApp builds the command tree the routing matrix runs against:
//
//	moln [--project/-p] [--verbose/-v] [--env]           group, no Run
//	├── deploy (alias ship) [--force/-f] [--region/-r] [--tag/-t ...]
//	│                       [--count/-n] [--name] [target]
//	├── project (alias proj) --org (required)           group
//	│   ├── ls (alias list) [--all/-a]
//	│   └── doctor                                       IsExtensionCommand
//	├── show [--project]                                 same name as the root's
//	├── calc [x] [y]                                     float positionals
//	├── db [name]                                        Run and children
//	│   └── migrate
//	├── cache                                            Run and children, no positionals
//	│   └── clear
//	├── mk --name (required)
//	├── secret                                           Hidden
//	└── version                                          IsExtensionCommand
type routingApp struct {
	app     *clix.App
	out     *strings.Builder
	ran     string
	initial map[string]string // "path --flag" -> Value.String() before the first Run
}

func newRoutingApp(t *testing.T) *routingApp {
	t.Helper()
	r := &routingApp{initial: map[string]string{}}
	app, _ := newTestApp(t, "moln")
	r.app = app
	run := func(ctx *clix.Context) error { r.ran = ctx.Command.Path(); return nil }
	str := func(fs *clix.FlagSet, fo clix.FlagOptions) { fs.StringVar(clix.StringVarOptions{FlagOptions: fo}) }
	boolean := func(fs *clix.FlagSet, fo clix.FlagOptions) { fs.BoolVar(clix.BoolVarOptions{FlagOptions: fo}) }

	str(app.Flags(), clix.FlagOptions{Name: "project", Short: "p"})
	boolean(app.Flags(), clix.FlagOptions{Name: "verbose", Short: "v"})
	str(app.Flags(), clix.FlagOptions{Name: "env"})

	deploy := clix.NewCommand("deploy", clix.WithCommandAliases("ship"), clix.WithCommandRun(run))
	boolean(deploy.Flags, clix.FlagOptions{Name: "force", Short: "f"})
	str(deploy.Flags, clix.FlagOptions{Name: "region", Short: "r"})
	deploy.Flags.StringSliceVar(clix.StringSliceVarOptions{FlagOptions: clix.FlagOptions{Name: "tag", Short: "t"}})
	deploy.Flags.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "count", Short: "n"}})
	str(deploy.Flags, clix.FlagOptions{Name: "name"})
	str(deploy.Flags, clix.FlagOptions{Name: "target", Positional: true})

	ls := clix.NewCommand("ls", clix.WithCommandAliases("list"), clix.WithCommandRun(run))
	boolean(ls.Flags, clix.FlagOptions{Name: "all", Short: "a"})
	doctor := clix.NewCommand("doctor", clix.WithCommandRun(run))
	doctor.IsExtensionCommand = true
	project := clix.NewGroup("project", "Projects", ls, doctor)
	project.Aliases = []string{"proj"}
	str(project.Flags, clix.FlagOptions{Name: "org", Required: true})

	show := clix.NewCommand("show", clix.WithCommandRun(run))
	str(show.Flags, clix.FlagOptions{Name: "project"})

	calc := clix.NewCommand("calc", clix.WithCommandRun(run))
	calc.Flags.Float64Var(clix.Float64VarOptions{FlagOptions: clix.FlagOptions{Name: "x", Positional: true}})
	calc.Flags.Float64Var(clix.Float64VarOptions{FlagOptions: clix.FlagOptions{Name: "y", Positional: true}})

	db := clix.NewCommand("db", clix.WithCommandRun(run))
	str(db.Flags, clix.FlagOptions{Name: "name", Positional: true})
	db.AddCommand(clix.NewCommand("migrate", clix.WithCommandRun(run)))

	cache := clix.NewCommand("cache", clix.WithCommandRun(run))
	cache.AddCommand(clix.NewCommand("clear", clix.WithCommandRun(run)))

	mk := clix.NewCommand("mk", clix.WithCommandRun(run))
	str(mk.Flags, clix.FlagOptions{Name: "name", Required: true})

	secret := clix.NewCommand("secret", clix.WithCommandHidden(true), clix.WithCommandRun(run))
	version := clix.NewCommand("version", clix.WithCommandRun(run))
	version.IsExtensionCommand = true

	for _, c := range []*clix.Command{deploy, project, show, calc, db, cache, mk, secret, version} {
		app.Root.AddCommand(c)
	}
	if err := app.Validate(); err != nil && !strings.Contains(err.Error(), "--project is also defined") {
		t.Fatalf("fixture: %v", err)
	}
	r.eachFlag(func(key string, f *clix.Flag) { r.initial[key] = f.Value.String() })
	r.out = &strings.Builder{}
	app.Out = r.out
	return r
}

// eachFlag calls fn for every flag in the tree except --help, keyed as
// "command path --name".
func (r *routingApp) eachFlag(fn func(key string, f *clix.Flag)) {
	var walk func(c *clix.Command)
	walk = func(c *clix.Command) {
		for _, f := range c.Flags.Flags() {
			if f.Name != "help" {
				fn(c.Path()+" --"+f.Name, f)
			}
		}
		for _, child := range c.Children {
			walk(child)
		}
	}
	walk(r.app.Root)
}

// Sources, abbreviated in the table.
const (
	cmdFlag = "@command"
	appFlag = "@app"
)

type routingCase struct {
	name string
	args []string
	ran  string            // command path whose Run ran, or ""
	help bool              // help was printed
	set  map[string]string // "path --flag" -> "value@source"; every other flag keeps its default
	err  string            // "" for nil, otherwise a substring of the error
	kind string            // "usage" or "missing" when err is set
}

func TestRoutingMatrix(t *testing.T) {
	const (
		deploy = "moln deploy"
		ls     = "moln project ls"
	)
	cases := []routingCase{
		// The command path: names, aliases, case, the root name.
		{name: "leaf", args: []string{"deploy"}, ran: deploy},
		{name: "alias", args: []string{"ship"}, ran: deploy},
		{name: "ASCII case", args: []string{"DePloy"}, ran: deploy},
		{name: "alias path", args: []string{"proj", "list", "--org", "o"}, ran: ls, set: map[string]string{"moln project --org": "o" + cmdFlag}},
		{name: "repeated root name", args: []string{"moln", "deploy"}, ran: deploy},
		{name: "bare root name", args: []string{"moln"}, help: true},
		{name: "positional equal to the root name", args: []string{"deploy", "moln"}, ran: deploy, set: map[string]string{deploy + " --target": "moln" + cmdFlag}},
		{name: "positional equal to a command name", args: []string{"deploy", "deploy"}, ran: deploy, set: map[string]string{deploy + " --target": "deploy" + cmdFlag}},
		{name: "hidden command", args: []string{"secret"}, ran: "moln secret"},
		{name: "no arguments", args: []string{}, help: true},
		{name: "group alone shows help", args: []string{"project"}, help: true},
		{name: "unknown command", args: []string{"nope"}, err: "unknown command: moln nope", kind: "usage"},
		{name: "unknown command in a group", args: []string{"project", "nope"}, err: "unknown command: moln project nope", kind: "usage"},
		{name: "help in the middle is a word", args: []string{"project", "help", "ls"}, err: "unknown command: moln project help", kind: "usage"},
		{name: "non-ASCII case does not fold", args: []string{"Kache"}, err: "unknown command", kind: "usage"},

		// Where flags may appear.
		{name: "root flag before the path", args: []string{"-p", "p1", "deploy"}, ran: deploy, set: map[string]string{"moln --project": "p1" + appFlag}},
		{name: "root flag after the path", args: []string{"deploy", "--project", "p1"}, ran: deploy, set: map[string]string{"moln --project": "p1" + appFlag}},
		{name: "flag value equal to a command name", args: []string{"--env", "deploy", "deploy"}, ran: deploy, set: map[string]string{"moln --env": "deploy" + appFlag}},
		{name: "group flag between path segments", args: []string{"project", "--org", "o", "ls"}, ran: ls, set: map[string]string{"moln project --org": "o" + cmdFlag}},
		{name: "group flag after the leaf", args: []string{"project", "ls", "--org", "o", "-a"}, ran: ls, set: map[string]string{"moln project --org": "o" + cmdFlag, ls + " --all": "true" + cmdFlag}},
		{name: "deeper flag before its command with =", args: []string{"--org=o", "project", "ls"}, ran: ls, set: map[string]string{"moln project --org": "o" + cmdFlag}},
		{name: "deeper switch before its command", args: []string{"--force", "deploy"}, ran: deploy, set: map[string]string{deploy + " --force": "true" + cmdFlag}},
		{name: "deeper flag before its command, value separate", args: []string{"--org", "o", "project", "ls"}, err: "unknown command: moln o", kind: "usage"},
		{name: "same name on root and leaf, after the leaf", args: []string{"show", "--project", "s1"}, ran: "moln show", set: map[string]string{"moln show --project": "s1" + cmdFlag}},
		{name: "same name on root and leaf, before the leaf", args: []string{"--project", "r1", "show"}, ran: "moln show", set: map[string]string{"moln --project": "r1" + appFlag}},

		// Values: =, separate, empty, repeated, slices, dashes.
		{name: "= value", args: []string{"deploy", "--region=eu"}, ran: deploy, set: map[string]string{deploy + " --region": "eu" + cmdFlag}},
		{name: "empty = value", args: []string{"deploy", "--name="}, ran: deploy, set: map[string]string{deploy + " --name": "" + cmdFlag}},
		{name: "repeated flag, last wins", args: []string{"deploy", "--name", "x", "--name", "y"}, ran: deploy, set: map[string]string{deploy + " --name": "y" + cmdFlag}},
		{name: "repeated slice flag appends", args: []string{"deploy", "-t", "a", "--tag", "b,c", "-td"}, ran: deploy, set: map[string]string{deploy + " --tag": "a,b,c,d" + cmdFlag}},
		{name: "value that looks like a flag", args: []string{"deploy", "--name", "--force"}, ran: deploy, set: map[string]string{deploy + " --name": "--force" + cmdFlag}},
		{name: "missing value", args: []string{"deploy", "--name"}, err: "flag --name requires a value", kind: "usage"},
		{name: "bad value", args: []string{"deploy", "--count", "1.5"}, err: "invalid value for count", kind: "usage"},
		{name: "lone dash is a positional", args: []string{"deploy", "-"}, ran: deploy, set: map[string]string{deploy + " --target": "-" + cmdFlag}},
		{name: "lone dash as a value", args: []string{"deploy", "--name", "-"}, ran: deploy, set: map[string]string{deploy + " --name": "-" + cmdFlag}},
		{name: "-- then a flag-like word", args: []string{"deploy", "--", "-f"}, ran: deploy, set: map[string]string{deploy + " --target": "-f" + cmdFlag}},
		{name: "-- then --help", args: []string{"deploy", "--", "--help"}, ran: deploy, set: map[string]string{deploy + " --target": "--help" + cmdFlag}},

		// Booleans never take the next token.
		{name: "bare switch", args: []string{"deploy", "--force"}, ran: deploy, set: map[string]string{deploy + " --force": "true" + cmdFlag}},
		{name: "switch then a word", args: []string{"deploy", "--force", "false"}, ran: deploy, set: map[string]string{deploy + " --force": "true" + cmdFlag, deploy + " --target": "false" + cmdFlag}},
		{name: "switch = false", args: []string{"deploy", "--force=false"}, ran: deploy, set: map[string]string{deploy + " --force": "false" + cmdFlag}},
		{name: "switch = empty", args: []string{"deploy", "--force="}, err: "invalid value for force", kind: "usage"},

		// Numbers.
		{name: "negative value, separate", args: []string{"deploy", "--count", "-5"}, ran: deploy, set: map[string]string{deploy + " --count": "-5" + cmdFlag}},
		{name: "negative value, =", args: []string{"deploy", "--count=-5"}, ran: deploy, set: map[string]string{deploy + " --count": "-5" + cmdFlag}},
		{name: "negative value, short", args: []string{"deploy", "-n", "-5"}, ran: deploy, set: map[string]string{deploy + " --count": "-5" + cmdFlag}},
		{name: "negative value, attached", args: []string{"deploy", "-n-5"}, ran: deploy, set: map[string]string{deploy + " --count": "-5" + cmdFlag}},
		{name: "negative positionals", args: []string{"calc", "-5", "-1.5"}, ran: "moln calc", set: map[string]string{"moln calc --x": "-5" + cmdFlag, "moln calc --y": "-1.5" + cmdFlag}},
		{name: "negative positionals after --", args: []string{"calc", "--", "-5", "-1.5"}, ran: "moln calc", set: map[string]string{"moln calc --x": "-5" + cmdFlag, "moln calc --y": "-1.5" + cmdFlag}},
		{name: "negative number as a string value", args: []string{"deploy", "--name", "-1.5"}, ran: deploy, set: map[string]string{deploy + " --name": "-1.5" + cmdFlag}},
		{name: "negative number in a group", args: []string{"-5"}, err: "unknown command: moln -5", kind: "usage"},

		// Short clusters and attached values, across the command chain.
		{name: "cluster", args: []string{"deploy", "-fv"}, ran: deploy, set: map[string]string{deploy + " --force": "true" + cmdFlag, "moln --verbose": "true" + appFlag}},
		{name: "attached value", args: []string{"deploy", "-rus"}, ran: deploy, set: map[string]string{deploy + " --region": "us" + cmdFlag}},
		{name: "cluster ending in a value flag", args: []string{"deploy", "-fr", "us"}, ran: deploy, set: map[string]string{deploy + " --force": "true" + cmdFlag, deploy + " --region": "us" + cmdFlag}},
		{name: "cluster with an attached value", args: []string{"deploy", "-vfrus"}, ran: deploy, set: map[string]string{"moln --verbose": "true" + appFlag, deploy + " --force": "true" + cmdFlag, deploy + " --region": "us" + cmdFlag}},
		{name: "cluster with an attached value after a value flag", args: []string{"deploy", "-frus"}, ran: deploy, set: map[string]string{deploy + " --force": "true" + cmdFlag, deploy + " --region": "us" + cmdFlag}},
		{name: "cluster with =", args: []string{"deploy", "-fr=us"}, ran: deploy, set: map[string]string{deploy + " --force": "true" + cmdFlag, deploy + " --region": "us" + cmdFlag}},
		{name: "cluster with an unknown letter", args: []string{"deploy", "-fz"}, err: "unknown flag: -fz", kind: "usage"},

		// Unknown flags and excess positionals.
		{name: "unknown flag on a command", args: []string{"deploy", "--bogus"}, err: "unknown flag: --bogus", kind: "usage"},
		{name: "unknown flag before the command", args: []string{"--bogus", "deploy"}, err: "unknown flag: --bogus", kind: "usage"},
		{name: "unknown flag on the non-strict root", args: []string{"--bogus"}, err: "unexpected arguments: --bogus", kind: "usage"},
		{name: "excess positionals", args: []string{"deploy", "a", "b"}, err: "unexpected arguments: b", kind: "usage"},

		// --help and -h.
		{name: "--help", args: []string{"--help"}, help: true},
		{name: "-h on a command", args: []string{"deploy", "-h"}, help: true},
		{name: "--help before the path", args: []string{"--help", "deploy"}, help: true},
		{name: "--help in the middle", args: []string{"project", "--help", "ls"}, help: true},
		{name: "--help beats required flags", args: []string{"mk", "--help"}, help: true},
		{name: "--help beats excess positionals", args: []string{"deploy", "a", "b", "--help"}, help: true},
		{name: "-h in a cluster", args: []string{"deploy", "-fh"}, help: true, set: map[string]string{deploy + " --force": "true" + cmdFlag}},

		// A command with both Run and children.
		{name: "Run and children, no argument", args: []string{"db"}, ran: "moln db"},
		{name: "Run and children, a child", args: []string{"db", "migrate"}, ran: "moln db migrate"},
		{name: "Run and children, another word", args: []string{"db", "users"}, ran: "moln db", set: map[string]string{"moln db --name": "users" + cmdFlag}},
		{name: "Run and children, no positionals", args: []string{"cache", "stuff"}, err: "unexpected arguments: stuff", kind: "usage"},

		// Required flags and extension commands.
		{name: "required flag missing", args: []string{"mk"}, err: "missing required flags: --name", kind: "missing"},
		{name: "required flag given", args: []string{"mk", "--name", "n"}, ran: "moln mk", set: map[string]string{"moln mk --name": "n" + cmdFlag}},
		{name: "group's required flag missing", args: []string{"project", "ls"}, err: "missing required flags: --org", kind: "missing"},
		{name: "extension command skips ancestors' required", args: []string{"project", "doctor"}, ran: "moln project doctor"},
		{name: "extension command at the root", args: []string{"version"}, ran: "moln version"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRoutingApp(t)
			// Run something else first: nothing may leak into the case.
			_ = r.app.Run(bg, []string{"deploy", "--force", "-p", "leak", "--name", "leak", "-t", "leak", "x"})
			r.ran, r.out = "", &strings.Builder{}
			r.app.Out = r.out

			err := r.app.Run(bg, c.args)

			if r.ran != c.ran {
				t.Errorf("ran %q, want %q (err %v)", r.ran, c.ran, err)
			}
			if printed := strings.Contains(r.out.String(), "USAGE"); printed != c.help {
				t.Errorf("help printed = %v, want %v", printed, c.help)
			}
			wantCode := 0
			switch {
			case c.err == "":
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
			case err == nil || !strings.Contains(err.Error(), c.err):
				t.Errorf("err = %v, want %q", err, c.err)
			default:
				wantCode = 2
				var missing *clix.MissingFlagsError
				if isMissing := errors.As(err, &missing); isMissing != (c.kind == "missing") {
					t.Errorf("err = %T %v, want kind %q", err, err, c.kind)
				}
			}
			if code := clix.ExitCode(err); code != wantCode {
				t.Errorf("ExitCode = %d, want %d", code, wantCode)
			}
			if err != nil {
				return // flag values after an error are unspecified
			}

			var problems []string
			r.eachFlag(func(key string, f *clix.Flag) {
				got := f.Value.String()
				src := map[clix.Source]string{clix.SourceCommandFlag: cmdFlag, clix.SourceAppFlag: appFlag}[f.Source()]
				if want, ok := c.set[key]; ok {
					if got+src != want {
						problems = append(problems, fmt.Sprintf("%s = %q%s, want %q", key, got, src, want))
					}
					return
				}
				if f.Source() != clix.SourceDefault || got != r.initial[key] {
					problems = append(problems, fmt.Sprintf("%s = %q from %v, want the default %q", key, got, f.Source(), r.initial[key]))
				}
			})
			sort.Strings(problems)
			for _, p := range problems {
				t.Error(p)
			}
		})
	}
}

// The root's name is skipped only when nothing else can use the word.
func TestRoutingRootNameWithRunAndPositional(t *testing.T) {
	app, _ := newTestApp(t, "echo")
	var text string
	app.Root.Run = noop
	app.Root.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "text", Positional: true}, Value: &text})
	if err := app.Run(bg, []string{"echo"}); err != nil || text != "echo" {
		t.Fatalf("echo echo: err = %v, text = %q", err, text)
	}
}

// A Hidden command runs but is left out of help.
func TestRoutingHiddenCommandNotInHelp(t *testing.T) {
	r := newRoutingApp(t)
	if err := r.app.Run(bg, []string{"--help"}); err != nil {
		t.Fatal(err)
	}
	if out := r.out.String(); strings.Contains(out, "secret") || !strings.Contains(out, "deploy") {
		t.Fatalf("help:\n%s", out)
	}
}
