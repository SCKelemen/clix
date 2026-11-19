package clix_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// Restoring went back to the value from before the first Run, so a value
// the program assigned between two Runs was discarded.
func TestValuesAssignedBetweenRunsWin(t *testing.T) {
	app, _ := newTestApp(t, "app")
	var name, got string
	var n int
	greet := clix.NewCommand("greet", clix.WithCommandRun(func(*clix.Context) error { got = fmt.Sprint(name, n); return nil }))
	greet.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "name"}, Value: &name})
	greet.Flags.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "n"}, Value: &n})
	app.Root.AddCommand(greet)

	steps := []struct {
		preset func()
		args   []string
		want   string
	}{
		{func() { name = "bob" }, []string{"greet"}, "bob0"},
		{func() { name = "alice" }, []string{"greet"}, "alice0"},
		{nil, []string{"greet", "--name", "carol", "--n", "3"}, "carol3"},
		{nil, []string{"greet"}, "alice0"}, // carol and 3 were set by the Run, so they are undone
		{func() { n = 7 }, []string{"greet", "--name", "dave"}, "dave7"},
		{func() { name = "erin" }, []string{"greet"}, "erin7"}, // the program's values win over what the Run set
	}
	for i, s := range steps {
		if s.preset != nil {
			s.preset()
		}
		if err := app.Run(bg, s.args); err != nil || got != s.want {
			t.Fatalf("step %d %q: err = %v, got %q, want %q", i, s.args, err, got, s.want)
		}
	}
}

// As the README says: a program's assignment between Runs is told apart
// from what the previous Run left only by its value, so assigning exactly
// that value is undone like the rest, and any other value is kept.
func TestAssignmentBetweenRunsIsToldByValue(t *testing.T) {
	app, _ := newTestApp(t, "app")
	var region string
	c := clix.NewCommand("c", clix.WithCommandRun(noop))
	c.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "region"}, Value: &region})
	app.Root.AddCommand(c)
	for _, step := range []struct {
		assign string
		want   string
	}{
		{"us", "us"}, // kept: it differs from the "eu" the Run left
		{"eu", ""},   // undone: indistinguishable from what the Run left
	} {
		region = ""
		if err := app.Run(bg, []string{"c", "--region", "eu"}); err != nil {
			t.Fatal(err)
		}
		region = step.assign
		if err := app.Run(bg, []string{"c"}); err != nil || region != step.want {
			t.Fatalf("assigned %q between Runs: %v, region = %q, want %q", step.assign, err, region, step.want)
		}
	}
}

// A Required flag satisfied by a value assigned before the second Run
// failed as missing.
func TestRequiredPresetBeforeSecondRun(t *testing.T) {
	app, _ := newTestApp(t, "app")
	var name string
	mk := clix.NewCommand("mk", clix.WithCommandRun(noop))
	mk.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "name", Required: true}, Value: &name})
	app.Root.AddCommand(mk)
	var missing *clix.MissingFlagsError
	if err := app.Run(bg, []string{"mk"}); !errors.As(err, &missing) {
		t.Fatalf("run 1 = %v", err)
	}
	name = "bob"
	if err := app.Run(bg, []string{"mk"}); err != nil {
		t.Fatalf("run 2 with name preset: %v", err)
	}
}

// resettableList is an accumulating Value with Reset.
type resettableList []string

func (l *resettableList) String() string     { return strings.Join(*l, ",") }
func (l *resettableList) Set(s string) error { *l = append(*l, s); return nil }
func (l *resettableList) Reset()             { *l = nil }

// plainList is the standard library's accumulating Value, without Reset.
type plainList []string

func (l *plainList) String() string     { return strings.Join(*l, ",") }
func (l *plainList) Set(s string) error { *l = append(*l, s); return nil }

// mode is an enum Value whose Set rejects its own zero text "".
type mode struct{ v string }

func (m *mode) String() string { return m.v }
func (m *mode) Set(s string) error {
	if s != "fast" && s != "slow" {
		return fmt.Errorf("must be fast or slow")
	}
	m.v = s
	return nil
}
func (m *mode) Reset() { m.v = "" }

// Restoring passed the saved text back to Set and ignored the error, so
// an accumulating list became ["a" "b" ""] and an enum that rejects ""
// kept the previous Run's value. Run only calls Reset: a preset the
// program gave the value is kept until a Run sets it, and a Default is
// applied again on every Run.
func TestCustomValuesWithReset(t *testing.T) {
	app, _ := newTestApp(t, "app")
	var list resettableList
	m := &mode{}
	preset := &mode{v: "slow"}
	def := &mode{}
	var seen string
	c := clix.NewCommand("c", clix.WithCommandRun(func(*clix.Context) error {
		seen = fmt.Sprintf("%q %q %q %q", []string(list), m.v, preset.v, def.v)
		return nil
	}))
	c.Flags.Var(clix.VarOptions{FlagOptions: clix.FlagOptions{Name: "x"}, Value: &list})
	c.Flags.Var(clix.VarOptions{FlagOptions: clix.FlagOptions{Name: "mode"}, Value: m})
	c.Flags.Var(clix.VarOptions{FlagOptions: clix.FlagOptions{Name: "preset"}, Value: preset})
	c.Flags.Var(clix.VarOptions{FlagOptions: clix.FlagOptions{Name: "def"}, Value: def, Default: "slow"})
	app.Root.AddCommand(c)
	for _, step := range []struct {
		args []string
		want string
	}{
		{[]string{"c"}, `[] "" "slow" "slow"`},
		{[]string{"c", "--x", "a", "--x", "b", "--mode", "fast", "--preset", "fast", "--def", "fast"}, `["a" "b"] "fast" "fast" "fast"`},
		{[]string{"c"}, `[] "" "" "slow"`},
		{[]string{"c", "--x", "z"}, `["z"] "" "" "slow"`},
		{[]string{"c"}, `[] "" "" "slow"`},
	} {
		if err := app.Run(bg, step.args); err != nil || seen != step.want {
			t.Fatalf("%q: err = %v, seen %s, want %s", step.args, err, seen, step.want)
		}
	}
	if err := app.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
}

// A custom Value without Reset cannot be put back: Set is not called (so
// it is not corrupted), and the carry-over is recorded for Err and
// Validate.
func TestCustomValueWithoutResetIsReported(t *testing.T) {
	app, _ := newTestApp(t, "app")
	var list plainList
	var seen []string
	c := clix.NewCommand("c", clix.WithCommandRun(func(*clix.Context) error { seen = append([]string(nil), list...); return nil }))
	c.Flags.Var(clix.VarOptions{FlagOptions: clix.FlagOptions{Name: "x"}, Value: &list})
	app.Root.AddCommand(c)
	if err := app.Run(bg, []string{"c", "--x", "a", "--x", "b"}); err != nil {
		t.Fatal(err)
	}
	if err := app.Validate(); err != nil {
		t.Fatalf("Validate after one Run = %v", err)
	}
	if err := app.Run(bg, []string{"c"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(seen, []string{"a", "b"}) {
		t.Fatalf("second run saw %q; want the carried-over [a b], not a corrupted list", seen)
	}
	want := "its value (*clix_test.plainList) has no Reset method"
	if err := c.Flags.Err(); err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("Err() = %v, want %q", err, want)
	}
	if err := app.Validate(); err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("Validate() = %v", err)
	}
}

// A default changed after registration to one the flag rejects is an
// error instead of being ignored.
func TestInvalidDefaultAtRunIsAnError(t *testing.T) {
	app, _ := newTestApp(t, "app")
	c := clix.NewCommand("c", clix.WithCommandRun(noop))
	c.Flags.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "port"}, Default: "80"})
	app.Root.AddCommand(c)
	c.Flags.Lookup("port").Default = "eighty"
	if err := app.Run(bg, []string{"c"}); err == nil || !strings.Contains(err.Error(), `invalid default "eighty" for --port`) {
		t.Fatalf("Run = %v", err)
	}
}

// The built-in values have Reset.
func TestBuiltinValuesReset(t *testing.T) {
	fs := clix.NewFlagSet("x")
	fs.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "s"}, Default: "v"})
	fs.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "b"}, Default: true})
	fs.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "i"}, Default: "1"})
	fs.Int64Var(clix.Int64VarOptions{FlagOptions: clix.FlagOptions{Name: "j"}, Default: "1"})
	fs.Float64Var(clix.Float64VarOptions{FlagOptions: clix.FlagOptions{Name: "f"}, Default: "1.5"})
	fs.DurationVar(clix.DurationVarOptions{FlagOptions: clix.FlagOptions{Name: "d"}, Default: "1s"})
	fs.StringSliceVar(clix.StringSliceVarOptions{FlagOptions: clix.FlagOptions{Name: "l"}, Default: []string{"a"}})
	for _, f := range fs.Flags() {
		r, ok := f.Value.(interface{ Reset() })
		if !ok {
			t.Fatalf("--%s: %T has no Reset", f.Name, f.Value)
		}
		r.Reset()
		if got := f.Value.String(); got != map[string]string{"s": "", "b": "false", "i": "0", "j": "0", "f": "0", "d": "0s", "l": ""}[f.Name] {
			t.Errorf("--%s after Reset = %q", f.Name, got)
		}
	}
}

// secretValue prints as <redacted> whatever it holds.
type secretValue struct{ v string }

func (s *secretValue) Set(x string) error { s.v = x; return nil }
func (s *secretValue) String() string     { return "<redacted>" }
func (s *secretValue) Reset()             { s.v = "" }

// labelsValue is a map Value whose String ("map[team:a]") is not text Set
// accepts.
type labelsValue map[string]string

func (m *labelsValue) Set(s string) error {
	k, v, ok := strings.Cut(s, "=")
	if !ok {
		return errors.New("want key=value")
	}
	if *m == nil {
		*m = labelsValue{}
	}
	(*m)[k] = v
	return nil
}
func (m *labelsValue) String() string { return fmt.Sprint(map[string]string(*m)) }
func (m *labelsValue) Reset()         { *m = nil }

// Whether a custom Value was set was decided by comparing String before
// and after the Run, so a secret that always prints <redacted> was never
// reset, and the next login without --token reused the old token.
func TestRedactedValueIsReset(t *testing.T) {
	app, _ := newTestApp(t, "app")
	tok := &secretValue{}
	var used string
	login := clix.NewCommand("login", clix.WithCommandRun(func(*clix.Context) error { used = tok.v; return nil }))
	login.Flags.Var(clix.VarOptions{FlagOptions: clix.FlagOptions{Name: "token"}, Value: tok})
	app.Root.AddCommand(login)
	for _, step := range []struct {
		args []string
		want string
	}{
		{[]string{"login", "--token", "s3cr3t"}, "s3cr3t"},
		{[]string{"login"}, ""},
		{[]string{"login", "--token", "other"}, "other"},
		{[]string{"login"}, ""},
	} {
		if err := app.Run(bg, step.args); err != nil || used != step.want {
			t.Fatalf("%q: err = %v, used %q, want %q", step.args, err, used, step.want)
		}
	}
	if err := app.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
}

// Undoing a custom Value called Reset and then Set with the old String,
// so a map whose String is "map[team:a]" failed the next Run. Run only
// calls Reset; a Default is how a value keeps a starting value.
func TestResetDoesNotNeedSettableString(t *testing.T) {
	app, _ := newTestApp(t, "app")
	plain := labelsValue{"team": "a"} // a preset, which Reset discards
	withDefault := labelsValue{}
	c := clix.NewCommand("c", clix.WithCommandRun(noop))
	c.Flags.Var(clix.VarOptions{FlagOptions: clix.FlagOptions{Name: "label"}, Value: &plain})
	c.Flags.Var(clix.VarOptions{FlagOptions: clix.FlagOptions{Name: "tag"}, Value: &withDefault, Default: "team=a"})
	app.Root.AddCommand(c)
	if err := app.Run(bg, []string{"c", "--label", "env=prod", "--tag", "env=prod"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := app.Run(bg, []string{"c"}); err != nil {
			t.Fatalf("run %d without flags: %v", i+2, err)
		}
		if len(plain) != 0 || fmt.Sprint(map[string]string(withDefault)) != "map[team:a]" {
			t.Fatalf("run %d: label = %v (want what Reset left), tag = %v (want the default)", i+2, plain, withDefault)
		}
	}
}

// A handler that calls Run (an alias command, a shell) made the outer
// Run's values leak into every later Run: the nested Run recorded them as
// its starting values, so nothing undid them.
func TestNestedRunDoesNotLeak(t *testing.T) {
	app, _ := newTestApp(t, "app")
	var force bool
	var name string
	var inner error
	rm := clix.NewCommand("rm", clix.WithCommandRun(func(ctx *clix.Context) error {
		inner = app.Run(ctx, []string{"ls"})
		return nil
	}))
	rm.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "force"}, Value: &force})
	rm.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "name"}, Value: &name})
	app.Root.AddCommand(rm)
	app.Root.AddCommand(clix.NewCommand("ls", clix.WithCommandRun(noop)))
	if err := app.Run(bg, []string{"rm", "--force", "--name", "x"}); err != nil || inner != nil {
		t.Fatalf("Run = %v, nested Run = %v", err, inner)
	}
	if err := app.Run(bg, []string{"ls"}); err != nil {
		t.Fatal(err)
	}
	if force || name != "" {
		t.Fatalf("a later ls left force=%v name=%q", force, name)
	}
	rm.Run = noop
	if err := app.Run(bg, []string{"rm"}); err != nil || force || name != "" {
		t.Fatalf("a later plain rm: %v, force=%v name=%q", err, force, name)
	}
}

// The nested Run cleared what the outer Run recorded, so after it the
// outer handler saw --force as not set and not Changed.
func TestNestedRunKeepsOuterState(t *testing.T) {
	app, _ := newTestApp(t, "app")
	var force bool
	var name string
	var inside, after string
	// The getters' view of --force and --name, then the bound variables.
	report := func(ctx *clix.Context) string {
		_, fsrc, fok := ctx.EffectiveBool("force")
		n, nsrc, _ := ctx.EffectiveString("name")
		return fmt.Sprintf("force=%v,%v,%v name=%q,%v,%v var=%v,%q", fok, fsrc, ctx.Command.Flags.Changed("force"), n, nsrc, ctx.Command.Flags.Changed("name"), force, name)
	}
	rm := clix.NewCommand("rm")
	rm.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "force"}, Value: &force})
	rm.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "name"}, Value: &name})
	rm.Run = func(ctx *clix.Context) error {
		if ctx.Command.Flags.Changed("force") {
			// The outer Run: run rm again, with other values.
			if err := app.Run(ctx, []string{"rm", "--name", "y"}); err != nil {
				return err
			}
			after = report(ctx)
			return nil
		}
		inside = report(ctx)
		return nil
	}
	app.Root.AddCommand(rm)
	if err := app.Run(bg, []string{"rm", "--force", "--name", "x"}); err != nil {
		t.Fatal(err)
	}
	if want := `force=true,command flag,true name="x",command flag,true var=true,"x"`; after != want {
		t.Errorf("after the nested Run the outer handler sees\n%s\nwant\n%s", after, want)
	}
	// The nested Run starts from the outer Run's values, with no flag set.
	if want := `force=false,command flag,false name="y",command flag,true var=true,"y"`; inside != want {
		t.Errorf("the nested Run sees\n%s\nwant\n%s", inside, want)
	}
	if err := app.Run(bg, []string{"rm", "--name", "z"}); err != nil || force || inside != `force=false,command flag,false name="z",command flag,true var=false,"z"` {
		t.Errorf("the next Run: %v, %s", err, inside)
	}
}

// Runs that a handler starts one after another, as a shell does, do not
// see each other's values, built-in or custom.
func TestNestedRunsInSequence(t *testing.T) {
	app, _ := newTestApp(t, "app")
	var region string
	labels := labelsValue{}
	var seen []string
	deploy := clix.NewCommand("deploy", clix.WithCommandRun(func(*clix.Context) error {
		seen = append(seen, fmt.Sprintf("%s %v", region, map[string]string(labels)))
		return nil
	}))
	deploy.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "region"}, Value: &region})
	deploy.Flags.Var(clix.VarOptions{FlagOptions: clix.FlagOptions{Name: "label"}, Value: &labels})
	shell := clix.NewCommand("shell", clix.WithCommandRun(func(ctx *clix.Context) error {
		for _, line := range []string{"deploy --region eu --label a=1", "deploy", "deploy --label b=2"} {
			if err := app.Run(ctx, strings.Fields(line)); err != nil {
				return err
			}
		}
		return nil
	}))
	app.Root.AddCommand(deploy)
	app.Root.AddCommand(shell)
	if err := app.Run(bg, []string{"shell"}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"eu map[a:1]", " map[]", " map[b:2]"}; !reflect.DeepEqual(seen, want) {
		t.Fatalf("seen %q, want %q", seen, want)
	}
	if region != "" || len(labels) != 0 {
		t.Fatalf("after the shell: region=%q labels=%v", region, labels)
	}
}
