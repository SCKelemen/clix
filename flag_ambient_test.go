package clix_test

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
)

// askPrompter answers each prompt with answers[label] and records the
// labels it was asked for, in order.
type askPrompter struct {
	answers map[string]string
	asked   []string
}

func (p *askPrompter) Prompt(_ context.Context, opts ...clix.PromptOption) (string, error) {
	var cfg clix.PromptConfig
	for _, o := range opts {
		o.Apply(&cfg)
	}
	p.asked = append(p.asked, cfg.Label)
	return p.answers[cfg.Label], nil
}

// ambientApp is a gcloud-style tree whose connection and output flags are
// Ambient:
//
//	moln [--target/-t] [--token] [--timeout] [--quiet/-q]   Ambient
//	     [--project/-p]                                      an input
//	└── widgets [--api-version]                              group, Ambient flag
//	    └── create --name (required) --size (required)
//	               [--wait/-w] (Ambient) [--dry-run/-n] [color] (positional)
type ambientApp struct {
	app      *clix.App
	prompter *askPrompter
	ran      bool
	target   string
	source   clix.Source // where --target came from
	name     string
	size     int
}

func newAmbientApp(t *testing.T, opts ...clix.AppOption) *ambientApp {
	t.Helper()
	t.Setenv("MOLN_DISABLE_PROMPTS", "")
	a := &ambientApp{prompter: &askPrompter{answers: map[string]string{"Name": "w1", "Size": "3"}}}
	opts = append([]clix.AppOption{clix.WithAppInteractive(clix.InteractiveAlways), clix.WithAppPrompter(a.prompter)}, opts...)
	app, _ := newTestApp(t, "moln", opts...)
	a.app = app

	root := app.Flags()
	root.StringVar(clix.WithFlagName("target"), clix.WithFlagShort("t"), clix.WithFlagAmbient(), clix.WithStringValue(&a.target))
	root.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "token", Ambient: true, Sensitive: true}})
	root.DurationVar(clix.DurationVarOptions{FlagOptions: clix.FlagOptions{Name: "timeout", Ambient: true}})
	root.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "quiet", Short: "q", Ambient: true}})
	root.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project", Short: "p"}})

	create := clix.NewCommand("create", clix.WithCommandRun(func(ctx *clix.Context) error {
		a.ran = true
		_, a.source, _ = ctx.EffectiveString("target")
		return nil
	}))
	create.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "name", Required: true}, Value: &a.name})
	create.Flags.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "size", Required: true}, Value: &a.size})
	create.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "wait", Short: "w", Ambient: true}})
	create.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "dry-run", Short: "n"}})
	create.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "color", Positional: true}})

	widgets := clix.NewGroup("widgets", "Manage widgets", create)
	widgets.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "api-version", Ambient: true}})
	app.Root.AddCommand(widgets)

	if err := app.Validate(); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return a
}

type ambientCase struct {
	name    string
	args    []string
	env     map[string]string
	asked   []string // labels prompted for, in order; nil when Run must not prompt
	missing []string // MissingFlagsError.Flags; nil when Run must succeed
	target  string   // --target after the Run, when not ""
}

var bothPrompts = []string{"Name", "Size"}

func (c ambientCase) run(t *testing.T, a *ambientApp) {
	t.Helper()
	for k, v := range c.env {
		t.Setenv(k, v)
	}
	err := a.app.Run(bg, c.args)
	if !reflect.DeepEqual(a.prompter.asked, c.asked) {
		t.Errorf("asked for %q, want %q (err %v)", a.prompter.asked, c.asked, err)
	}
	if c.missing != nil {
		var missing *clix.MissingFlagsError
		if !errors.As(err, &missing) || !reflect.DeepEqual(missing.Flags, c.missing) {
			t.Fatalf("err = %T %v, want a *MissingFlagsError for %q", err, err, c.missing)
		}
		if code := clix.ExitCode(err); code != 2 {
			t.Errorf("ExitCode = %d, want 2", code)
		}
		if a.ran {
			t.Error("the command ran")
		}
		return
	}
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !a.ran {
		t.Fatal("the command did not run")
	}
	if c.asked != nil && (a.name != "w1" || a.size != 3) {
		t.Errorf("name = %q, size = %d; want the prompted w1 and 3", a.name, a.size)
	}
	if c.target != "" && (a.target != c.target || a.source != clix.SourceAppFlag) {
		t.Errorf("--target = %q from %v, want %q from the command line", a.target, a.source, c.target)
	}
}

// Ambient flags do not count as "flags passed": wherever and however they
// are written, Run still prompts for missing required flags. Any flag that
// is not Ambient still turns prompting off.
func TestAmbientFlagsDoNotStopPrompting(t *testing.T) {
	const create = "create"
	cases := []ambientCase{
		{name: "no flags", args: []string{"widgets", create}, asked: bothPrompts},

		// Only Ambient flags: Run prompts.
		{name: "root flag before the path", args: []string{"--target", "X", "widgets", create}, asked: bothPrompts, target: "X"},
		{name: "root flag between path segments", args: []string{"widgets", "--target", "X", create}, asked: bothPrompts, target: "X"},
		{name: "root flag after the path", args: []string{"widgets", create, "--target", "X"}, asked: bothPrompts, target: "X"},
		{name: "--flag=value before the path", args: []string{"--target=X", "widgets", create}, asked: bothPrompts, target: "X"},
		{name: "--flag=value after the path", args: []string{"widgets", create, "--target=X"}, asked: bothPrompts, target: "X"},
		{name: "short flag", args: []string{"-t", "X", "widgets", create}, asked: bothPrompts, target: "X"},
		{name: "short flag with an attached value", args: []string{"widgets", create, "-tX"}, asked: bothPrompts, target: "X"},
		{name: "short flag with =", args: []string{"widgets", create, "-t=X"}, asked: bothPrompts, target: "X"},
		{name: "cluster of root and command switches", args: []string{"widgets", create, "-qw"}, asked: bothPrompts},
		{name: "cluster before the path", args: []string{"-qt=X", "widgets", create}, asked: bothPrompts, target: "X"},
		{name: "cluster ending in a value flag", args: []string{"widgets", create, "-wqt", "X"}, asked: bothPrompts, target: "X"},
		{name: "cluster with an attached value", args: []string{"widgets", create, "-qtX"}, asked: bothPrompts, target: "X"},
		{name: "group flag", args: []string{"widgets", "--api-version", "v2", create}, asked: bothPrompts},
		{name: "group flag after the leaf", args: []string{"widgets", create, "--api-version=v2"}, asked: bothPrompts},
		{name: "command flag", args: []string{"widgets", create, "--wait"}, asked: bothPrompts},
		{name: "switch set to false", args: []string{"widgets", create, "--wait=false"}, asked: bothPrompts},
		{name: "every kind at once", args: []string{"--target", "X", "--token=s3cret", "--timeout", "5s", "-q", "widgets", "--api-version", "v2", create, "-w"}, asked: bothPrompts, target: "X"},
		{name: "repeated", args: []string{"-t", "X", "widgets", create, "--target", "Y"}, asked: bothPrompts, target: "Y"},

		// Environment variables never counted, Ambient or not.
		{name: "input flag from the environment", args: []string{"widgets", create}, env: map[string]string{"MOLN_PROJECT": "p1"}, asked: bothPrompts},
		{name: "Ambient flag from the environment", args: []string{"widgets", create}, env: map[string]string{"MOLN_TARGET": "E"}, asked: bothPrompts},

		// Required flags given with Ambient flags: nothing to prompt for.
		{name: "required flags given", args: []string{"--target", "X", "widgets", create, "--name", "w1", "--size", "3"}, target: "X"},
		{name: "one required flag given", args: []string{"--target", "X", "widgets", create, "--name", "w2"}, missing: []string{"size"}},

		// Mixed with a flag that is not Ambient: no prompt.
		{name: "command input flag", args: []string{"--target", "X", "widgets", create, "--dry-run"}, missing: []string{"name", "size"}},
		{name: "root input flag", args: []string{"--target", "X", "--project", "p", "widgets", create}, missing: []string{"name", "size"}},
		{name: "root input flag after the leaf", args: []string{"widgets", create, "-tX", "-p", "p"}, missing: []string{"name", "size"}},
		{name: "cluster of Ambient and input switches", args: []string{"widgets", create, "-wn"}, missing: []string{"name", "size"}},
		{name: "cluster of a root Ambient switch and an input switch", args: []string{"widgets", create, "-qn"}, missing: []string{"name", "size"}},
		{name: "cluster with an input switch before the path", args: []string{"-nq", "widgets", create}, missing: []string{"name", "size"}},
		{name: "input switch set to false", args: []string{"widgets", create, "-w", "--dry-run=false"}, missing: []string{"name", "size"}},
		{name: "positional argument", args: []string{"--target", "X", "widgets", create, "blue"}, missing: []string{"name", "size"}},
		{name: "positional argument after --", args: []string{"-q", "widgets", create, "--", "--target"}, missing: []string{"name", "size"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := newAmbientApp(t)
			// Run something else first: nothing may leak into the case.
			if err := a.app.Run(bg, []string{"widgets", "create", "--name", "x", "--size", "1", "--dry-run", "red"}); err != nil {
				t.Fatal(err)
			}
			a.ran, a.prompter.asked, a.name, a.size = false, nil, "", 0
			c.run(t, a)
		})
	}
}

// Without anyone to answer, Ambient flags change nothing: Run returns a
// *MissingFlagsError, as for no flags at all.
func TestAmbientFlagsWhenNotInteractive(t *testing.T) {
	cases := []struct {
		name string
		opts []clix.AppOption
		env  map[string]string
	}{
		{name: "InteractiveNever", opts: []clix.AppOption{clix.WithAppInteractive(clix.InteractiveNever)}},
		{name: "InteractiveAuto with a pipe", opts: []clix.AppOption{clix.WithAppInteractive(clix.InteractiveAuto), clix.WithAppIn(strings.NewReader("w1\n3\n"))}},
		{name: "DISABLE_PROMPTS", env: map[string]string{"MOLN_DISABLE_PROMPTS": "1"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := newAmbientApp(t, c.opts...)
			ambientCase{args: []string{"--target", "X", "widgets", "create"}, env: c.env, missing: []string{"name", "size"}}.run(t, a)
		})
	}
}

// An Ambient flag can be Required: it is prompted for like any other, and
// given on the command line it satisfies the requirement without turning
// prompting off for the others.
func TestAmbientRequiredFlag(t *testing.T) {
	newApp := func(t *testing.T) (*clix.App, *askPrompter, *string, *string) {
		p := &askPrompter{answers: map[string]string{"Region": "eu", "Name": "w1"}}
		app, _ := newTestApp(t, "moln", clix.WithAppInteractive(clix.InteractiveAlways), clix.WithAppPrompter(p))
		var region, name string
		app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "region", Required: true, Ambient: true}, Value: &region})
		cmd := clix.NewCommand("create", clix.WithCommandRun(noop))
		cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "name", Required: true}, Value: &name})
		app.Root.AddCommand(cmd)
		return app, p, &region, &name
	}

	app, p, region, name := newApp(t)
	if err := app.Run(bg, []string{"create"}); err != nil || !reflect.DeepEqual(p.asked, []string{"Region", "Name"}) || *region != "eu" || *name != "w1" {
		t.Fatalf("create: err = %v, asked %q, region %q, name %q", err, p.asked, *region, *name)
	}

	app, p, region, name = newApp(t)
	if err := app.Run(bg, []string{"--region", "us", "create"}); err != nil || !reflect.DeepEqual(p.asked, []string{"Name"}) || *region != "us" || *name != "w1" {
		t.Fatalf("--region us create: err = %v, asked %q, region %q, name %q", err, p.asked, *region, *name)
	}

	// The required check itself is unchanged: with an input flag given,
	// a missing Ambient flag is reported, not prompted for.
	app, p, _, _ = newApp(t)
	err := app.Run(bg, []string{"create", "--name", "w1"})
	var missing *clix.MissingFlagsError
	if !errors.As(err, &missing) || !reflect.DeepEqual(missing.Flags, []string{"region"}) || len(p.asked) != 0 {
		t.Fatalf("create --name w1: err = %v, asked %q", err, p.asked)
	}
}

// An Ambient flag set by position does not count either.
func TestAmbientPositionalFlag(t *testing.T) {
	p := &askPrompter{answers: map[string]string{"Name": "w1"}}
	app, _ := newTestApp(t, "moln", clix.WithAppInteractive(clix.InteractiveAlways), clix.WithAppPrompter(p))
	var region, name string
	cmd := clix.NewCommand("create", clix.WithCommandRun(noop))
	cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "region", Positional: true, Ambient: true}, Value: &region})
	cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "name", Required: true}, Value: &name})
	app.Root.AddCommand(cmd)
	if err := app.Run(bg, []string{"create", "eu"}); err != nil || !reflect.DeepEqual(p.asked, []string{"Name"}) || region != "eu" || name != "w1" {
		t.Fatalf("create eu: err = %v, asked %q, region %q, name %q", err, p.asked, region, name)
	}
}

// Ambient changes only the prompting decision: the flag is still read from
// the environment and the config file, honors NoEnv, and FlagSet.AnyCLISet
// still reports it.
func TestAmbientFlagOtherwiseUnchanged(t *testing.T) {
	a := newAmbientApp(t)
	t.Setenv("MOLN_TARGET", "from-env")
	if err := a.app.Run(bg, []string{"widgets", "create", "--name", "n", "--size", "1"}); err != nil {
		t.Fatal(err)
	}
	if a.target != "from-env" || a.source != clix.SourceEnvVar {
		t.Errorf("--target = %q from %v, want from-env from the environment", a.target, a.source)
	}

	a = newAmbientApp(t)
	t.Setenv("MOLN_TARGET", "")
	writeConfigFile(t, a.app, "target: from-config\n")
	if err := a.app.Run(bg, []string{"widgets", "create", "--name", "n", "--size", "1"}); err != nil {
		t.Fatal(err)
	}
	if a.target != "from-config" || a.source != clix.SourceConfigFile {
		t.Errorf("--target = %q from %v, want from-config from the config file", a.target, a.source)
	}

	a = newAmbientApp(t)
	if err := a.app.Run(bg, []string{"-t", "X", "widgets", "create"}); err != nil {
		t.Fatal(err)
	}
	if !a.app.Flags().AnyCLISet() || !a.app.Flags().Changed("target") {
		t.Error("AnyCLISet or Changed(target) = false after -t X")
	}

	// A bad value is a usage error, as for any flag.
	a = newAmbientApp(t)
	if err := a.app.Run(bg, []string{"--timeout", "soon", "widgets", "create"}); err == nil || clix.ExitCode(err) != 2 || len(a.prompter.asked) != 0 {
		t.Errorf("--timeout soon: err = %v, asked %q", err, a.prompter.asked)
	}
}

// Help shows an Ambient flag exactly as it shows the same flag without
// Ambient, and Validate has nothing to say about it.
func TestAmbientFlagHelpUnchanged(t *testing.T) {
	help := func(ambient bool) string {
		out := &bytes.Buffer{}
		app, _ := newTestApp(t, "moln", clix.WithAppOut(out))
		app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "target", Short: "t", Usage: "API endpoint", Ambient: ambient}})
		cmd := clix.NewCommand("create", clix.WithCommandRun(noop))
		cmd.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "wait", Usage: "Wait until done", Ambient: ambient}})
		app.Root.AddCommand(cmd)
		if err := app.Validate(); err != nil {
			t.Fatalf("Validate (Ambient %v) = %v", ambient, err)
		}
		if err := app.Run(bg, []string{"create", "--help"}); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	plain, ambient := help(false), help(true)
	if plain != ambient || !strings.Contains(ambient, "--target") || !strings.Contains(ambient, "--wait") {
		t.Fatalf("help differs or lacks the flags:\nwithout Ambient:\n%s\nwith Ambient:\n%s", plain, ambient)
	}
}

// endpoint is a custom Value for FlagSet.Var.
type endpoint struct{ s string }

func (e *endpoint) Set(s string) error { e.s = s; return nil }
func (e *endpoint) String() string     { return e.s }
func (e *endpoint) Reset()             { e.s = "" }

// FlagOptions.Ambient and WithFlagAmbient reach the Flag for every flag
// type, in the struct, pointer, functional and mixed styles.
func TestAmbientOptionOnEveryFlagType(t *testing.T) {
	fo := clix.FlagOptions{Name: "x", Ambient: true}
	var (
		s  string
		b  bool
		i  int
		i6 int64
		f  float64
		d  time.Duration
		ss []string
	)
	register := map[string]func(fs *clix.FlagSet){
		"string":            func(fs *clix.FlagSet) { fs.StringVar(clix.StringVarOptions{FlagOptions: fo, Value: &s}) },
		"*string":           func(fs *clix.FlagSet) { fs.StringVar(&clix.StringVarOptions{FlagOptions: fo, Value: &s}) },
		"bool":              func(fs *clix.FlagSet) { fs.BoolVar(clix.BoolVarOptions{FlagOptions: fo, Value: &b}) },
		"int":               func(fs *clix.FlagSet) { fs.IntVar(clix.IntVarOptions{FlagOptions: fo, Value: &i}) },
		"int64":             func(fs *clix.FlagSet) { fs.Int64Var(clix.Int64VarOptions{FlagOptions: fo, Value: &i6}) },
		"float64":           func(fs *clix.FlagSet) { fs.Float64Var(clix.Float64VarOptions{FlagOptions: fo, Value: &f}) },
		"duration":          func(fs *clix.FlagSet) { fs.DurationVar(clix.DurationVarOptions{FlagOptions: fo, Value: &d}) },
		"string slice":      func(fs *clix.FlagSet) { fs.StringSliceVar(clix.StringSliceVarOptions{FlagOptions: fo, Value: &ss}) },
		"var":               func(fs *clix.FlagSet) { fs.Var(clix.VarOptions{FlagOptions: fo, Value: &endpoint{}}) },
		"FlagOptions value": func(fs *clix.FlagSet) { fs.StringVar(fo, clix.WithStringValue(&s)) },
		"functional":        func(fs *clix.FlagSet) { fs.BoolVar(clix.WithFlagName("x"), clix.WithFlagAmbient()) },
		"mixed": func(fs *clix.FlagSet) {
			fs.IntVar(clix.WithFlagAmbient(), clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "x"}, Value: &i})
		},
	}
	for name, reg := range register {
		t.Run(name, func(t *testing.T) {
			fs := clix.NewFlagSet("t")
			reg(fs)
			if f := fs.Lookup("x"); f == nil || !f.Ambient {
				t.Fatalf("flag = %+v, want Ambient", f)
			}
		})
	}

	fs := clix.NewFlagSet("t")
	fs.StringVar(clix.WithFlagName("plain"))
	if fs.Lookup("plain").Ambient {
		t.Error("a flag is Ambient by default")
	}
}
