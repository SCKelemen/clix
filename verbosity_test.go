package clix_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
)

// extendFunc is an Extension made of a function.
type extendFunc func(*clix.App) error

func (f extendFunc) Extend(app *clix.App) error { return f(app) }

// newVerbosityApp returns an app with --verbosity, captured stdout and
// stderr, no prompting, and a "log" command that logs one record at each
// level (debug, info, warning, error, critical).
func newVerbosityApp(t *testing.T) (app *clix.App, out, errOut *bytes.Buffer) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MYAPP_VERBOSITY", "")
	out, errOut = &bytes.Buffer{}, &bytes.Buffer{}
	app = clix.NewApp("myapp", clix.WithAppOut(out), clix.WithAppErr(errOut), clix.WithAppInteractive(clix.InteractiveNever))
	app.EnableVerbosity()
	app.Root.AddCommand(clix.NewCommand("log", clix.WithCommandRun(func(ctx *clix.Context) error {
		l := ctx.Logger()
		l.Debug("d")
		l.Info("i")
		l.Warn("w")
		l.Error("e")
		l.Log(ctx, clix.LevelCritical, "c")
		return nil
	})))
	return app, out, errOut
}

func TestEnableVerbosityRegistersFlag(t *testing.T) {
	app, _, _ := newVerbosityApp(t)
	app.EnableVerbosity() // a second call does nothing
	f := app.Flags().Lookup("verbosity")
	if f == nil {
		t.Fatal("no --verbosity flag")
	}
	n := 0
	for _, g := range app.Flags().Flags() {
		if g.Name == "verbosity" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d --verbosity flags after two EnableVerbosity calls", n)
	}
	if !f.Ambient || f.CommandLineOnly || f.Short != "" || f.Default != "warning" || f.EnvVar != "MYAPP_VERBOSITY" || f.ConfigKey != "verbosity" {
		t.Fatalf("flag = %+v", f)
	}
	if key, ok := app.FlagConfigKey(f); !ok || key != "verbosity" {
		t.Fatalf("FlagConfigKey = %q, %v", key, ok)
	}
	var values []string
	for _, c := range f.Completer()(clix.CompletionRequest{Flag: f}) {
		values = append(values, c.Value)
	}
	if want := []string{"debug", "info", "warning", "error", "critical", "none"}; !reflect.DeepEqual(values, want) {
		t.Fatalf("completion = %v, want %v", values, want)
	}
	if err := app.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestVerbosityLevelsFilter(t *testing.T) {
	all := []string{"DEBUG: d", "INFO: i", "WARNING: w", "ERROR: e", "CRITICAL: c"}
	for _, tc := range []struct {
		verbosity string
		want      []string
	}{
		{"debug", all},
		{"info", all[1:]},
		{"warning", all[2:]},
		{"error", all[3:]},
		{"critical", all[4:]},
		{"none", nil},
		{"", all[2:]}, // the default
	} {
		t.Run(tc.verbosity, func(t *testing.T) {
			app, out, errOut := newVerbosityApp(t)
			args := []string{"log"}
			if tc.verbosity != "" {
				args = []string{"--verbosity", tc.verbosity, "log"}
			}
			if err := app.Run(bg, args); err != nil {
				t.Fatal(err)
			}
			want := ""
			if len(tc.want) > 0 {
				want = strings.Join(tc.want, "\n") + "\n"
			}
			if errOut.String() != want {
				t.Fatalf("App.Err = %q, want %q", errOut.String(), want)
			}
			if out.Len() != 0 {
				t.Fatalf("logs reached App.Out: %q", out.String())
			}
		})
	}
}

func TestVerbosityLevelValues(t *testing.T) {
	for name, want := range map[string]slog.Level{
		"debug":    slog.LevelDebug,
		"info":     slog.LevelInfo,
		"warning":  slog.LevelWarn,
		"error":    slog.LevelError,
		"critical": clix.LevelCritical,
		"none":     clix.LevelNone,
		"DEBUG":    slog.LevelDebug, // case is ignored
		"Warning":  slog.LevelWarn,
	} {
		app, _, _ := newVerbosityApp(t)
		var got slog.Level
		app.Root.AddCommand(clix.NewCommand("level", clix.WithCommandRun(func(ctx *clix.Context) error {
			got = ctx.App.LogLevel().Level()
			return nil
		})))
		if err := app.Run(bg, []string{"--verbosity=" + name, "level"}); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got != want {
			t.Errorf("--verbosity=%s: level %v, want %v", name, got, want)
		}
		if v, _ := app.Flags().String("verbosity"); v != strings.ToLower(name) {
			t.Errorf("--verbosity=%s reads back as %q", name, v)
		}
	}
	if clix.LevelCritical <= slog.LevelError || clix.LevelNone <= clix.LevelCritical {
		t.Fatal("levels out of order")
	}
}

func TestVerbosityPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name                string
		flag, env, config   string
		want                slog.Level
		wantSource          clix.Source
		autoEnv, autoConfig bool
	}{
		{name: "default", want: slog.LevelWarn, wantSource: clix.SourceDefault, autoEnv: true, autoConfig: true},
		{name: "config", config: "info", want: slog.LevelInfo, wantSource: clix.SourceConfigFile, autoEnv: true, autoConfig: true},
		{name: "env over config", env: "error", config: "info", want: slog.LevelError, wantSource: clix.SourceEnvVar, autoEnv: true, autoConfig: true},
		{name: "flag over env", flag: "debug", env: "error", config: "info", want: slog.LevelDebug, wantSource: clix.SourceAppFlag, autoEnv: true, autoConfig: true},
		{name: "env with AutoEnv off", env: "Critical", want: clix.LevelCritical, wantSource: clix.SourceEnvVar},
		{name: "config with AutoConfig off", config: "none", want: clix.LevelNone, wantSource: clix.SourceConfigFile},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, _, _ := newVerbosityApp(t)
			if !tc.autoEnv {
				off := false
				app.AutoEnv = &off
			}
			if !tc.autoConfig {
				off := false
				app.AutoConfig = &off
			}
			t.Setenv("MYAPP_VERBOSITY", tc.env)
			if tc.config != "" {
				writeConfigFile(t, app, "verbosity: "+tc.config+"\n")
			}
			var got slog.Level
			var source clix.Source
			app.Root.AddCommand(clix.NewCommand("level", clix.WithCommandRun(func(ctx *clix.Context) error {
				got = ctx.App.LogLevel().Level()
				_, source, _ = ctx.EffectiveString("verbosity")
				return nil
			})))
			args := []string{"level"}
			if tc.flag != "" {
				args = []string{"--verbosity", tc.flag, "level"}
			}
			if err := app.Run(bg, args); err != nil {
				t.Fatal(err)
			}
			if got != tc.want || source != tc.wantSource {
				t.Fatalf("level %v from %v, want %v from %v", got, source, tc.want, tc.wantSource)
			}
		})
	}
}

func TestVerbosityInvalidIsUsageError(t *testing.T) {
	const levels = "valid levels: debug, info, warning, error, critical, none"
	for _, tc := range []struct {
		name       string
		args       []string
		env        string
		config     string
		wantSource string
	}{
		{name: "flag", args: []string{"--verbosity", "loud", "log"}, wantSource: "invalid value for verbosity"},
		{name: "empty flag", args: []string{"--verbosity=", "log"}, wantSource: "invalid value for verbosity"},
		{name: "env", args: []string{"log"}, env: "loud", wantSource: "environment variable MYAPP_VERBOSITY"},
		{name: "config", args: []string{"log"}, config: "verbosity: loud\n", wantSource: `config file key "verbosity"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, _, errOut := newVerbosityApp(t)
			t.Setenv("MYAPP_VERBOSITY", tc.env)
			if tc.config != "" {
				writeConfigFile(t, app, tc.config)
			}
			err := app.Run(bg, tc.args)
			if err == nil || clix.ExitCode(err) != 2 || !strings.Contains(err.Error(), levels) || !strings.Contains(err.Error(), tc.wantSource) {
				t.Fatalf("err = %v (exit %d), want a usage error naming %s and the %s", err, clix.ExitCode(err), tc.wantSource, levels)
			}
			if errOut.Len() != 0 {
				t.Fatalf("the command ran and logged: %q", errOut.String())
			}
		})
	}
}

func TestVerbosityIsAmbient(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MYAPP_VERBOSITY", "")
	t.Setenv("MYAPP_DISABLE_PROMPTS", "")
	errOut := &bytes.Buffer{}
	app := clix.NewApp("myapp",
		clix.WithAppIn(strings.NewReader("w1\n")),
		clix.WithAppOut(&bytes.Buffer{}),
		clix.WithAppErr(errOut),
		clix.WithAppInteractive(clix.InteractiveAlways))
	app.EnableVerbosity()
	var name string
	create := clix.NewCommand("create", clix.WithCommandRun(func(ctx *clix.Context) error {
		ctx.Logger().Debug("creating", "name", name)
		return nil
	}))
	create.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "name", Required: true}, Value: &name})
	app.Root.AddCommand(clix.NewGroup("widgets", "Manage widgets", create))

	if err := app.Run(bg, []string{"--verbosity", "debug", "widgets", "create"}); err != nil {
		t.Fatal(err)
	}
	if name != "w1" {
		t.Fatalf("name = %q: --verbosity stopped the prompt", name)
	}
	if !strings.Contains(errOut.String(), "Name") || !strings.HasSuffix(errOut.String(), "DEBUG: creating name=w1\n") {
		t.Fatalf("App.Err = %q, want the prompt then the debug line", errOut.String())
	}
}

func TestVerbosityAppFlagWins(t *testing.T) {
	// The app's own --verbosity wins over the one an extension enables:
	// it is not a log level, and the logger stays at warning.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MYAPP_VERBOSITY", "")
	errOut := &bytes.Buffer{}
	app := clix.NewApp("myapp", clix.WithAppErr(errOut), clix.WithAppOut(&bytes.Buffer{}), clix.WithAppInteractive(clix.InteractiveNever))
	var mine int
	app.Flags().IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "verbosity"}, Value: &mine})
	app.AddExtension(extendFunc(func(a *clix.App) error { a.EnableVerbosity(); return nil }))
	app.Root.AddCommand(clix.NewCommand("log", clix.WithCommandRun(func(ctx *clix.Context) error {
		ctx.Logger().Info("i")
		ctx.Logger().Warn("w")
		return nil
	})))
	if err := app.Run(bg, []string{"--verbosity", "3", "log"}); err != nil {
		t.Fatal(err)
	}
	if mine != 3 || errOut.String() != "WARNING: w\n" {
		t.Fatalf("app flag = %d, App.Err = %q", mine, errOut.String())
	}
	if f := app.Flags().Lookup("verbosity"); f.Completer() != nil {
		t.Fatal("the app's --verbosity got the level completion")
	}
	if err := app.Validate(); err == nil || !strings.Contains(err.Error(), "--verbosity") {
		t.Fatalf("Validate = %v, want the replaced flag reported", err)
	}
}

func TestVerbosityLoggerFollowsErr(t *testing.T) {
	app, _, first := newVerbosityApp(t)
	if err := app.Run(bg, []string{"log"}); err != nil {
		t.Fatal(err)
	}
	second := &bytes.Buffer{}
	app.Err = second
	if err := app.Run(bg, []string{"--verbosity", "error", "log"}); err != nil {
		t.Fatal(err)
	}
	if first.String() != "WARNING: w\nERROR: e\nCRITICAL: c\n" || second.String() != "ERROR: e\nCRITICAL: c\n" {
		t.Fatalf("first Err %q, second Err %q", first.String(), second.String())
	}
}

func TestVerbosityLevelPerRun(t *testing.T) {
	app, _, errOut := newVerbosityApp(t)
	var inner, after slog.Level
	app.Root.AddCommand(clix.NewCommand("outer", clix.WithCommandRun(func(ctx *clix.Context) error {
		if err := ctx.App.Run(ctx, []string{"--verbosity", "error", "inner"}); err != nil {
			return err
		}
		after = ctx.App.LogLevel().Level()
		return nil
	})))
	app.Root.AddCommand(clix.NewCommand("inner", clix.WithCommandRun(func(ctx *clix.Context) error {
		inner = ctx.App.LogLevel().Level()
		return nil
	})))
	if err := app.Run(bg, []string{"--verbosity", "debug", "outer"}); err != nil {
		t.Fatal(err)
	}
	if inner != slog.LevelError || after != slog.LevelDebug {
		t.Fatalf("nested Run level %v, outer level after it %v; want ERROR then DEBUG", inner, after)
	}
	// The level stays until the next Run, which starts from the default.
	if got := app.LogLevel().Level(); got != slog.LevelDebug {
		t.Fatalf("level after Run = %v", got)
	}
	if err := app.Run(bg, []string{"log"}); err != nil {
		t.Fatal(err)
	}
	if got := app.LogLevel().Level(); got != slog.LevelWarn || errOut.String() != "WARNING: w\nERROR: e\nCRITICAL: c\n" {
		t.Fatalf("next Run: level %v, App.Err %q", got, errOut.String())
	}
}

func TestLoggerBeforeRunAndWithoutVerbosity(t *testing.T) {
	errOut := &bytes.Buffer{}
	app := clix.NewApp("myapp", clix.WithAppErr(errOut))
	if app.LogLevel().Level() != slog.LevelWarn {
		t.Fatalf("level before Run = %v", app.LogLevel().Level())
	}
	if _, ok := app.LogLevel().(*slog.LevelVar); ok {
		t.Fatal("LogLevel exposes a settable *slog.LevelVar")
	}
	if first, second := app.Logger(), app.Logger(); first != second {
		t.Fatal("Logger returns a new logger each time")
	}
	app.Logger().Info("hidden")
	app.Logger().Warn("shown")
	if errOut.String() != "WARNING: shown\n" {
		t.Fatalf("App.Err = %q", errOut.String())
	}

	// Without EnableVerbosity there is no flag, and Run keeps warning.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MYAPP_VERBOSITY", "debug")
	app.Root.AddCommand(clix.NewCommand("noop", clix.WithCommandRun(noop)))
	if err := app.Run(bg, []string{"noop"}); err != nil {
		t.Fatal(err)
	}
	if app.Flags().Lookup("verbosity") != nil || app.LogLevel().Level() != slog.LevelWarn {
		t.Fatalf("without EnableVerbosity: level %v", app.LogLevel().Level())
	}
	var nilApp *clix.App
	if nilApp.Logger() == nil || nilApp.LogLevel().Level() != slog.LevelWarn {
		t.Fatal("nil App")
	}
}

func TestContextLoggerWithoutApp(t *testing.T) {
	for _, ctx := range []*clix.Context{nil, {Context: bg}} {
		l := ctx.Logger()
		if l == nil {
			t.Fatal("nil logger")
		}
		if l.Enabled(bg, slog.LevelInfo) || !l.Enabled(bg, slog.LevelWarn) {
			t.Fatal("the logger of a Context without an App is not at warning")
		}
	}
}

func TestLoggerOutputFormat(t *testing.T) {
	app, _, errOut := newVerbosityApp(t)
	app.Root.AddCommand(clix.NewCommand("fmt", clix.WithCommandRun(func(ctx *clix.Context) error {
		l := ctx.Logger()
		l.Info("fetched page", "project", "my project", "items", 3, "ok", true)
		l.Info("")
		l.Info("no attributes")
		l.Info("built-in names", "msg", "kept", "level", "kept too", "time", 1)
		l.With("request", 7).WithGroup("http").Info("done", "status", 200, slog.Group("timing", "ms", 12))
		l.WithGroup("empty").Info("a group without attributes")
		l.With("a", 1).With("b", 2).Debug("chained", "c", "x=y")
		l.Info("two\nlines", "v", "tab\there")
		l.Log(ctx, slog.LevelWarn+1, "custom level")
		l.Log(ctx, clix.LevelCritical, "critical")
		return nil
	})))
	if err := app.Run(bg, []string{"--verbosity=debug", "fmt"}); err != nil {
		t.Fatal(err)
	}
	want := `INFO: fetched page project="my project" items=3 ok=true
INFO:
INFO: no attributes
INFO: built-in names msg=kept level="kept too" time=1
INFO: done request=7 http.status=200 http.timing.ms=12
INFO: a group without attributes
DEBUG: chained a=1 b=2 c="x=y"
INFO: "two\nlines" v="tab\there"
WARN+1: custom level
CRITICAL: critical
`
	if errOut.String() != want {
		t.Fatalf("App.Err:\n%s\nwant:\n%s", errOut.String(), want)
	}
}

func TestLoggerConcurrent(t *testing.T) {
	app, _, errOut := newVerbosityApp(t)
	const n = 64
	app.Root.AddCommand(clix.NewCommand("many", clix.WithCommandRun(func(ctx *clix.Context) error {
		var wg sync.WaitGroup
		for i := range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				l := ctx.Logger().With("worker", i)
				l.WithGroup("g").Info("working", "step", i)
			}()
		}
		wg.Wait()
		return nil
	})))
	if err := app.Run(bg, []string{"--verbosity", "info", "many"}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(errOut.String(), "\n"), "\n")
	if len(lines) != n {
		t.Fatalf("%d lines, want %d:\n%s", len(lines), n, errOut.String())
	}
	seen := map[string]bool{}
	for _, line := range lines {
		var worker, step int
		if _, err := fmt.Sscanf(line, "INFO: working worker=%d g.step=%d", &worker, &step); err != nil || worker != step {
			t.Fatalf("garbled line %q", line)
		}
		seen[line] = true
	}
	if len(seen) != n {
		t.Fatalf("%d distinct lines, want %d", len(seen), n)
	}
}

func TestVerbosityInHelpAndEnvironmentTopic(t *testing.T) {
	app, out, _ := newVerbosityApp(t)
	if err := app.Run(bg, []string{"--help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "--verbosity") || !strings.Contains(out.String(), `(default "warning")`) {
		t.Fatalf("help does not show --verbosity and its default:\n%s", out.String())
	}
	env, ok := app.LookupHelpTopic("environment")
	if !ok || !strings.Contains(env.Body, "MYAPP_VERBOSITY") || !strings.Contains(env.Body, "--verbosity: Log verbosity") {
		t.Fatalf("environment topic:\n%s", env.Body)
	}
}

// The logger's level can feed another handler, which then filters the
// same way.
func TestLogLevelForOtherHandlers(t *testing.T) {
	app, _, _ := newVerbosityApp(t)
	var json bytes.Buffer
	app.Root.AddCommand(clix.NewCommand("json", clix.WithCommandRun(func(ctx *clix.Context) error {
		l := slog.New(slog.NewJSONHandler(&json, &slog.HandlerOptions{Level: ctx.App.LogLevel()}))
		l.Debug("hidden")
		l.Error("shown")
		return nil
	})))
	if err := app.Run(bg, []string{"--verbosity", "info", "json"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(json.String(), "hidden") || !strings.Contains(json.String(), `"msg":"shown"`) {
		t.Fatalf("JSON handler output %q", json.String())
	}
}

// A handler error is returned to the caller of Handle.
func TestLoggerWriteError(t *testing.T) {
	app := clix.NewApp("myapp", clix.WithAppErr(failingWriter{}))
	h := app.Logger().Handler()
	r := slog.NewRecord(time.Time{}, slog.LevelError, "x", 0)
	if err := h.Handle(context.Background(), r); !errors.Is(err, errWrite) {
		t.Fatalf("Handle = %v", err)
	}
}

var errWrite = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }
