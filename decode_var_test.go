package clix_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

type deploySpec struct {
	Image    string            `json:"image"`
	Replicas int               `json:"replicas"`
	Version  string            `json:"version"`
	Env      map[string]string `json:"env"`
	Secret   secretText        `json:"secret"`
}

// secretText rejects values without a prefix, quoting the value, as a
// careless UnmarshalText might.
type secretText string

func (s *secretText) UnmarshalText(b []byte) error {
	if !strings.HasPrefix(string(b), "sk_") {
		return errors.New("bad secret " + string(b))
	}
	*s = secretText(b)
	return nil
}

// decodeApp has "deploy --spec/-f FILE", decoding into spec, whose
// Replicas defaults to 1.
type decodeApp struct {
	app    *clix.App
	deploy *clix.Command
	spec   deploySpec
}

func newDecodeApp(t *testing.T, sensitive bool, opts ...clix.AppOption) *decodeApp {
	t.Helper()
	a := &decodeApp{spec: deploySpec{Replicas: 1}}
	a.app, _ = newTestApp(t, "myapp", opts...)
	a.app.EnableFlagsFile()
	a.deploy = clix.NewCommand("deploy", clix.WithCommandRun(noop))
	a.deploy.Flags.DecodeVar(clix.DecodeVarOptions{
		FlagOptions: clix.FlagOptions{Name: "spec", Short: "f", Sensitive: sensitive},
		Value:       &a.spec,
	})
	a.app.Root.AddCommand(a.deploy)
	return a
}

func TestDecodeVar(t *testing.T) {
	dir := t.TempDir()
	yamlSpec := writeTestFile(t, dir, "spec.yaml", "image: web:1\nversion: 1.10\nenv: {A: '1'}\n")
	jsonSpec := writeTestFile(t, dir, "spec.json", `{"image": "web:2", "replicas": 3}`)

	tests := []struct {
		name  string
		args  []string
		stdin string
		want  deploySpec
	}{
		{"YAML, defaults kept", []string{"deploy", "--spec", yamlSpec}, "", deploySpec{Image: "web:1", Replicas: 1, Version: "1.10", Env: map[string]string{"A": "1"}}},
		{"JSON", []string{"deploy", "-f", jsonSpec}, "", deploySpec{Image: "web:2", Replicas: 3}},
		{"a leading @ is ignored", []string{"deploy", "--spec=@" + jsonSpec}, "", deploySpec{Image: "web:2", Replicas: 3}},
		{"standard input", []string{"deploy", "-f", "-"}, "image: piped\n", deploySpec{Image: "piped", Replicas: 1}},
		{"@- is standard input", []string{"deploy", "-f", "@-"}, `{"replicas": 7}`, deploySpec{Replicas: 7}},
		{"repeated: each file in turn", []string{"deploy", "-f", yamlSpec, "-f", jsonSpec}, "", deploySpec{Image: "web:2", Replicas: 3, Version: "1.10", Env: map[string]string{"A": "1"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newDecodeApp(t, false, clix.WithAppIn(strings.NewReader(tt.stdin)))
			if err := a.app.Run(context.Background(), tt.args); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(a.spec, tt.want) {
				t.Errorf("got  %+v\nwant %+v", a.spec, tt.want)
			}
			if !a.deploy.Flags.Changed("spec") {
				t.Error("Changed(spec) is false")
			}
		})
	}
}

// "@@name" names the file "@name", relative to the current directory.
func TestDecodeVarOddNames(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "@odd.yaml", "image: odd\n")
	t.Chdir(dir)
	a := newDecodeApp(t, false)
	if err := a.app.Run(context.Background(), []string{"deploy", "-f", "@@odd.yaml"}); err != nil {
		t.Fatal(err)
	}
	if a.spec.Image != "odd" {
		t.Errorf("spec = %+v", a.spec)
	}
	if got, _ := a.deploy.Flags.String("spec"); got != "@odd.yaml" {
		t.Errorf("String = %q", got)
	}
}

// A file with problems is a usage error that lists them all, and the
// variable is not changed.
func TestDecodeVarErrors(t *testing.T) {
	dir := t.TempDir()
	bad := writeTestFile(t, dir, "bad.yaml", "image: web\nimage: again\nreplicas: three\nimg: x\n")
	tests := []struct {
		name  string
		args  []string
		stdin string
		limit int64
		want  []string
	}{
		{"strict problems", []string{"deploy", "-f", bad}, "", 0, []string{"invalid value for spec: " + bad + ": strict decoding error: ", `duplicate field "image"`, `unknown field "img"`, "replicas: expected int, got a string"}},
		{"a missing file", []string{"deploy", "-f", filepath.Join(dir, "nope.yaml")}, "", 0, []string{"--spec:", "nope.yaml"}},
		{"no file name", []string{"deploy", "--spec="}, "", 0, []string{"name a file, or - for standard input"}},
		{"bad YAML", []string{"deploy", "-f", "-"}, "image: [x\n", 0, []string{"standard input: yaml: line"}},
		{"over the size limit", []string{"deploy", "-f", "-"}, "image: web\n", 4, []string{"standard input is larger than 4 bytes"}},
		{"standard input twice", []string{"deploy", "-f", "-", "-f", "-"}, "image: web\n", 0, []string{"standard input was already read by --spec"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newDecodeApp(t, false, clix.WithAppIn(strings.NewReader(tt.stdin)))
			a.app.MaxFileSize = tt.limit
			err := a.app.Run(context.Background(), tt.args)
			if err == nil || clix.ExitCode(err) != 2 {
				t.Fatalf("err = %v, want a usage error", err)
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
			if tt.name != "standard input twice" && !reflect.DeepEqual(a.spec, deploySpec{Replicas: 1}) {
				t.Errorf("the variable changed: %+v", a.spec)
			}
		})
	}
	t.Run("errors.As finds the StrictError", func(t *testing.T) {
		a := newDecodeApp(t, false)
		err := a.app.Run(context.Background(), []string{"deploy", "-f", bad})
		var strict *clix.StrictError
		if !errors.As(err, &strict) || len(strict.Fields) != 3 || !errors.Is(err, clix.ErrUnknownField) {
			t.Fatalf("err = %v", err)
		}
	})
}

// A Sensitive DecodeVar's errors name fields but never show values.
func TestDecodeVarSensitive(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "creds.yaml", "secret: hunter2-SECRET\nreplicas: SECRET-two\nbogus: SECRET-3\n")
	for _, sensitive := range []bool{false, true} {
		a := newDecodeApp(t, sensitive)
		err := a.app.Run(context.Background(), []string{"deploy", "-f", path})
		if err == nil {
			t.Fatal("expected an error")
		}
		msg := err.Error()
		for _, w := range []string{`unknown field "bogus"`, "replicas: expected int, got a string", "secret: "} {
			if !strings.Contains(msg, w) {
				t.Errorf("sensitive=%v: %q does not contain %q", sensitive, msg, w)
			}
		}
		if leaked := strings.Contains(msg, "SECRET"); leaked == sensitive {
			t.Errorf("sensitive=%v: %q", sensitive, msg)
		}
	}
}

// DecodeVar reads from every source: the command line beats an
// environment variable (a file name), which beats the config file, whose
// key may hold the document itself.
func TestDecodeVarSources(t *testing.T) {
	dir := t.TempDir()
	cliFile := writeTestFile(t, dir, "cli.yaml", "image: from-cli\n")
	envFile := writeTestFile(t, dir, "env.yaml", "image: from-env\n")
	cfgFile := writeTestFile(t, dir, "cfg.yaml", "image: from-config-file\n")
	tests := []struct {
		name       string
		args       []string
		env        string
		config     string // the config file's text
		wantImage  string
		wantSource clix.Source
	}{
		{"command line", []string{"deploy", "-f", cliFile}, envFile, "spec:\n  image: from-config\n", "from-cli", clix.SourceCommandFlag},
		{"environment", []string{"deploy"}, envFile, "spec:\n  image: from-config\n", "from-env", clix.SourceEnvVar},
		{"a config section", []string{"deploy"}, "", "spec:\n  image: from-config\n  version: 1.10\n", "from-config", clix.SourceConfigFile},
		{"a config file name", []string{"deploy"}, "", "spec: " + cfgFile + "\n", "from-config-file", clix.SourceConfigFile},
		{"nothing", []string{"deploy"}, "", "", "", clix.SourceDefault},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newDecodeApp(t, false)
			if tt.env != "" {
				t.Setenv("MYAPP_SPEC", tt.env)
			}
			if tt.config != "" {
				a.app.ConfigPath = writeTestFile(t, t.TempDir(), "config.yaml", tt.config)
			}
			if err := a.app.Run(context.Background(), tt.args); err != nil {
				t.Fatal(err)
			}
			if a.spec.Image != tt.wantImage || a.deploy.Flags.Lookup("spec").Source() != tt.wantSource {
				t.Errorf("image = %q, source = %v", a.spec.Image, a.deploy.Flags.Lookup("spec").Source())
			}
			if tt.name == "a config section" && a.spec.Version != "1.10" {
				t.Errorf("version = %q", a.spec.Version)
			}
		})
	}

	t.Run("a config section with problems", func(t *testing.T) {
		a := newDecodeApp(t, false)
		a.app.ConfigPath = writeTestFile(t, t.TempDir(), "config.yaml", "spec:\n  imag: x\n")
		err := a.app.Run(context.Background(), []string{"deploy"})
		if err == nil || clix.ExitCode(err) != 2 || !strings.Contains(err.Error(), `unknown field "spec.imag"`) {
			t.Fatalf("err = %v", err)
		}
	})
}

// In a flags file, a DecodeVar file name is relative to the flags file.
func TestDecodeVarInFlagsFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "conf/spec.yaml", "image: beside-flags\n")
	flags := writeTestFile(t, dir, "conf/flags.yaml", "--spec: spec.yaml\n")
	a := newDecodeApp(t, false)
	if err := a.app.Run(context.Background(), []string{"deploy", "--flags-file", flags}); err != nil {
		t.Fatal(err)
	}
	if a.spec.Image != "beside-flags" {
		t.Errorf("spec = %+v", a.spec)
	}
}

// A second Run puts the variable back, and a Required DecodeVar prompts
// for a file name.
func TestDecodeVarRunTwiceAndPrompt(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "spec.yaml", "image: web\nreplicas: 4\n")
	a := newDecodeApp(t, false)
	if err := a.app.Run(context.Background(), []string{"deploy", "-f", path}); err != nil {
		t.Fatal(err)
	}
	if err := a.app.Run(context.Background(), []string{"deploy"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.spec, deploySpec{Replicas: 1}) {
		t.Errorf("after a second Run: %+v", a.spec)
	}

	app, _ := newTestApp(t, "myapp", clix.WithAppIn(strings.NewReader(path+"\n")), clix.WithAppInteractive(clix.InteractiveAlways))
	var spec deploySpec
	cmd := clix.NewCommand("deploy", clix.WithCommandRun(noop))
	cmd.Flags.DecodeVar(clix.WithFlagName("spec"), clix.WithFlagRequired(), clix.WithDecodeValue(&spec))
	app.Root.AddCommand(cmd)
	if err := app.Run(context.Background(), []string{"deploy"}); err != nil {
		t.Fatal(err)
	}
	if spec.Image != "web" || cmd.Flags.Lookup("spec").Source() != clix.SourcePrompt {
		t.Errorf("spec = %+v, source = %v", spec, cmd.Flags.Lookup("spec").Source())
	}
}

// DecodeVar needs a pointer to decode into.
func TestDecodeVarNeedsAPointer(t *testing.T) {
	for _, v := range []any{nil, deploySpec{}, (*deploySpec)(nil)} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%T: no panic", v)
				}
			}()
			clix.NewFlagSet("x").DecodeVar(clix.DecodeVarOptions{FlagOptions: clix.FlagOptions{Name: "spec"}, Value: v})
		}()
	}
}

// FlagSet.Parse decodes outside Run, and Set does too.
func TestDecodeVarOutsideRun(t *testing.T) {
	path := writeTestFile(t, t.TempDir(), "spec.yaml", "image: parsed\n")
	var spec deploySpec
	fs := clix.NewFlagSet("x")
	fs.DecodeVar(clix.DecodeVarOptions{FlagOptions: clix.FlagOptions{Name: "spec"}, Value: &spec})
	if _, err := fs.Parse([]string{"--spec", path}); err != nil || spec.Image != "parsed" {
		t.Fatalf("spec = %+v, err = %v", spec, err)
	}
	spec = deploySpec{}
	if err := fs.Lookup("spec").Value.Set(path); err != nil || spec.Image != "parsed" {
		t.Fatalf("Set: spec = %+v, err = %v", spec, err)
	}
}

func TestDecodeVarInlineScalarList(t *testing.T) {
	for input, want := range map[string][]string{
		"[]": {}, "[one, 'two,three']": {"one", "two,three"},
	} {
		t.Run(input, func(t *testing.T) {
			app, _ := newTestApp(t, "myapp")
			app.ConfigPath = writeTestFile(t, t.TempDir(), "config.yaml", "items: "+input)
			items := []string{"default"}
			app.Flags().DecodeVar(clix.DecodeVarOptions{FlagOptions: clix.FlagOptions{Name: "items"}, Value: &items})
			app.Root.Run = noop
			if err := app.Run(t.Context(), []string{}); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(items, want) {
				t.Errorf("items = %#v, want %#v", items, want)
			}
		})
	}
}
