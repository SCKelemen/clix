package clix_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
)

// For every type and source, the Context getter reports the value the
// bound variable holds, with the right Source.
func TestGetterMatrix(t *testing.T) {
	raws := map[string]string{"int": "7", "int64": "7", "float64": "0.5", "bool": "true", "duration": "90s"}
	for _, typ := range []string{"int", "int64", "float64", "bool", "duration"} {
		for _, s := range []string{"cli", "env", "config", "default"} {
			t.Run(typ+"/"+s, func(t *testing.T) {
				app, _ := newTestApp(t, "moln")
				raw := raws[typ]
				var got, bound string
				var src clix.Source
				var ok bool
				var i int
				var i64 int64
				var f float64
				var b bool
				var d time.Duration
				cmd := clix.NewCommand("ls", clix.WithCommandRun(func(ctx *clix.Context) error {
					switch typ {
					case "int":
						var v int
						v, src, ok = ctx.EffectiveInt("x")
						w, _ := ctx.Int("x")
						got, bound = fmt.Sprint(v, w), fmt.Sprint(i, i)
					case "int64":
						var v int64
						v, src, ok = ctx.EffectiveInt64("x")
						w, _ := ctx.Int64("x")
						got, bound = fmt.Sprint(v, w), fmt.Sprint(i64, i64)
					case "float64":
						var v float64
						v, src, ok = ctx.EffectiveFloat64("x")
						w, _ := ctx.Float64("x")
						got, bound = fmt.Sprint(v, w), fmt.Sprint(f, f)
					case "bool":
						var v bool
						v, src, ok = ctx.EffectiveBool("x")
						got, bound = fmt.Sprint(v), fmt.Sprint(b)
					case "duration":
						var v time.Duration
						v, src, ok = ctx.EffectiveDuration("x")
						w, _ := ctx.Duration("x")
						got, bound = fmt.Sprint(v, w), fmt.Sprint(d, d)
					}
					return nil
				}))
				def := ""
				if s == "default" {
					def = raw
				}
				fo := clix.FlagOptions{Name: "x"}
				switch typ {
				case "int":
					cmd.Flags.IntVar(clix.IntVarOptions{FlagOptions: fo, Default: def, Value: &i})
				case "int64":
					cmd.Flags.Int64Var(clix.Int64VarOptions{FlagOptions: fo, Default: def, Value: &i64})
				case "float64":
					cmd.Flags.Float64Var(clix.Float64VarOptions{FlagOptions: fo, Default: def, Value: &f})
				case "bool":
					cmd.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: fo, Default: def == "true", Value: &b})
				case "duration":
					cmd.Flags.DurationVar(clix.DurationVarOptions{FlagOptions: fo, Default: def, Value: &d})
				}
				app.Root.AddCommand(cmd)
				args := []string{"ls"}
				switch s {
				case "cli":
					args = append(args, "--x="+raw)
				case "env":
					t.Setenv("MOLN_X", raw)
				case "config":
					writeConfigFile(t, app, "x: "+raw+"\n")
				}
				if err := app.Run(bg, args); err != nil {
					t.Fatal(err)
				}
				wantSrc := map[string]clix.Source{"cli": clix.SourceCommandFlag, "env": clix.SourceEnvVar, "config": clix.SourceConfigFile, "default": clix.SourceDefault}[s]
				if !ok || got != bound || src != wantSrc || bound == "" {
					t.Errorf("getter = %q from %v (ok=%v), bound = %q, want source %v", got, src, ok, bound, wantSrc)
				}
				if fl := cmd.Flags.Lookup("x"); fl.Source() != wantSrc {
					t.Errorf("Flag.Source() = %v, want %v", fl.Source(), wantSrc)
				}
				if cmd.Flags.Changed("x") != (s == "cli") {
					t.Errorf("Changed(x) = %v", cmd.Flags.Changed("x"))
				}
			})
		}
	}
}

func TestBoolDefaultTrue(t *testing.T) {
	app, _ := newTestApp(t, "moln")
	var color bool
	cmd := clix.NewCommand("ls", clix.WithCommandRun(noop))
	cmd.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "color"}, Default: true, Value: &color})
	cmd.Flags.BoolVar(clix.WithFlagName("other"), clix.WithBoolDefault(true))
	app.Root.AddCommand(cmd)
	for _, c := range []struct {
		args []string
		want bool
	}{{[]string{"ls"}, true}, {[]string{"ls", "--color=false"}, false}, {[]string{"ls"}, true}} {
		if err := app.Run(bg, c.args); err != nil || color != c.want {
			t.Fatalf("Run(%q) = %v, color = %v", c.args, err, color)
		}
	}
	if v, _ := cmd.Flags.Bool("other"); !v {
		t.Fatal("WithBoolDefault(true) ignored")
	}
}

// level is a custom Value.
type level int

func (l *level) Set(s string) error {
	switch strings.ToLower(s) {
	case "debug":
		*l = 0
	case "info":
		*l = 1
	case "warn":
		*l = 2
	default:
		return fmt.Errorf("unknown level %q", s)
	}
	return nil
}

func (l *level) String() string { return [...]string{"debug", "info", "warn"}[*l] }

// toggle is a custom switch using the standard library's IsBoolFlag
// convention.
type toggle struct{ on bool }

func (t *toggle) Set(s string) error {
	t.on = s == "true"
	return nil
}
func (t *toggle) String() string   { return fmt.Sprint(t.on) }
func (t *toggle) IsBoolFlag() bool { return true }
func (t *toggle) Reset()           { t.on = false }

func TestVarCustomValue(t *testing.T) {
	app, _ := newTestApp(t, "moln")
	var lv level
	tg := &toggle{}
	var viaCtx string
	cmd := clix.NewCommand("ls", clix.WithCommandRun(func(ctx *clix.Context) error {
		viaCtx, _ = ctx.String("level")
		return nil
	}))
	cmd.Flags.Var(clix.VarOptions{FlagOptions: clix.FlagOptions{Name: "level", Short: "l"}, Default: "info", Value: &lv})
	cmd.Flags.Var(clix.WithFlagName("dry-run"), clix.WithValue(tg))
	app.Root.AddCommand(cmd)

	if err := app.Run(bg, []string{"ls", "-l", "warn", "--dry-run", "extra"}); err == nil {
		t.Fatal("a switch consumed the next token")
	}
	if err := app.Run(bg, []string{"ls", "-l", "warn", "--dry-run"}); err != nil || lv != 2 || !tg.on || viaCtx != "warn" {
		t.Fatalf("err = %v, level = %v, dry-run = %v, ctx = %q", err, lv, tg.on, viaCtx)
	}
	if err := app.Run(bg, []string{"ls"}); err != nil || lv != 1 || tg.on {
		t.Fatalf("second run: err = %v, level = %v, dry-run = %v", err, lv, tg.on)
	}
	if err := app.Run(bg, []string{"ls", "--level", "loud"}); clix.ExitCode(err) != 2 {
		t.Fatalf("bad level: %v", err)
	}
}

func TestStringSliceVar(t *testing.T) {
	app, _ := newTestApp(t, "moln")
	var tags []string
	cmd := clix.NewCommand("ls", clix.WithCommandRun(noop))
	cmd.Flags.StringSliceVar(clix.StringSliceVarOptions{FlagOptions: clix.FlagOptions{Name: "tag", Short: "t"}, Default: []string{"default"}, Value: &tags})
	app.Root.AddCommand(cmd)

	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"ls"}, []string{"default"}},
		{[]string{"ls", "--tag", "a", "-t", "b,c", "--tag=\"d,e\""}, []string{"a", "b", "c", "d,e"}},
		{[]string{"ls", "--tag", "x"}, []string{"x"}},
		{[]string{"ls"}, []string{"default"}},
	}
	for _, c := range cases {
		if err := app.Run(bg, c.args); err != nil || !reflect.DeepEqual(tags, c.want) {
			t.Fatalf("Run(%q) = %v, tags = %q, want %q", c.args, err, tags, c.want)
		}
	}
	if got, _ := cmd.Flags.StringSlice("tag"); !reflect.DeepEqual(got, []string{"default"}) {
		t.Fatalf("StringSlice = %q", got)
	}

	t.Setenv("MOLN_TAG", "e1,e2")
	if err := app.Run(bg, []string{"ls"}); err != nil || !reflect.DeepEqual(tags, []string{"e1", "e2"}) {
		t.Fatalf("env: %v, tags = %q", err, tags)
	}
	t.Setenv("MOLN_TAG", "")

	writeConfigFile(t, app, "tag: [c1, c2]\n")
	app2, _ := newTestApp(t, "moln")
	var tags2 []string
	cmd2 := clix.NewCommand("ls", clix.WithCommandRun(noop))
	cmd2.Flags.StringSliceVar(clix.WithFlagName("tag"), clix.WithStringSliceValue(&tags2), clix.WithStringSliceDefault("d"))
	app2.Root.AddCommand(cmd2)
	writeConfigFile(t, app2, "tag: [c1, c2]\n")
	if err := app2.Run(bg, []string{"ls"}); err != nil || !reflect.DeepEqual(tags2, []string{"c1", "c2"}) {
		t.Fatalf("config: %v, tags = %q", err, tags2)
	}
}

func TestFlagSetDurationAndLookup(t *testing.T) {
	fs := clix.NewFlagSet("x")
	fs.DurationVar(clix.DurationVarOptions{FlagOptions: clix.FlagOptions{Name: "timeout"}, Default: "5s"})
	if d, ok := fs.Duration("timeout"); !ok || d != 5*time.Second {
		t.Fatalf("Duration = %v, %v", d, ok)
	}
	if _, ok := fs.Duration("missing"); ok {
		t.Fatal("Duration(missing) ok")
	}
	if fs.Lookup("timeout") == nil || fs.Lookup("missing") != nil {
		t.Fatal("Lookup")
	}
	if _, err := fs.Parse([]string{"--timeout", "1m"}); err != nil || !fs.Changed("timeout") || fs.Lookup("timeout").Source() != clix.SourceCommandFlag {
		t.Fatalf("Parse: %v changed=%v", err, fs.Changed("timeout"))
	}
}
