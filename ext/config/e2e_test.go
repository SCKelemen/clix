package config

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

func TestConfigE2E_BareConfigShowsHelp(t *testing.T) {
	app, out, path := e2eApp(t, "")
	got := run(t, app, out, "config")
	for _, want := range []string{"Manage CLI configuration", "probe config [command]", "list", "get", "set", "unset", "reset"} {
		if !strings.Contains(got, want) {
			t.Errorf("config help lacks %q:\n%s", want, got)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("config help created the config file (%v)", err)
	}
}

// A missing key is an error (exit code 1) naming the key.
func TestConfigE2E_GetMissingKey(t *testing.T) {
	app, out, _ := e2eApp(t, "")
	got, err := tryRun(app, out, "config", "get", "no.such.key")
	if err == nil || err.Error() != `config key "no.such.key" not found` || clix.ExitCode(err) != 1 || got != "" {
		t.Errorf("config get no.such.key: out %q, err %v, exit code %d", got, err, clix.ExitCode(err))
	}
	// Without a key there is nobody to ask: a usage error.
	_, err = tryRun(app, out, "config", "get")
	if err == nil || clix.ExitCode(err) != 2 {
		t.Errorf("config get without a key: err %v, exit code %d, want 2", err, clix.ExitCode(err))
	}
}

func TestConfigE2E_SetAndGetNestedKeys(t *testing.T) {
	app, out, path := e2eApp(t, "")
	if got := run(t, app, out, "config", "set", "project.default.region", "eu-west-1"); got != "project.default.region = eu-west-1\n" {
		t.Errorf("config set printed %q", got)
	}
	run(t, app, out, "config", "set", "--key", "project.name", "--value", "demo")
	if got := run(t, app, out, "config", "get", "project.default.region"); got != "eu-west-1\n" {
		t.Errorf("config get printed %q", got)
	}
	want := "project:\n  default:\n    region: eu-west-1\n  name: demo\n"
	if got := run(t, app, out, "config", "list"); got != want {
		t.Errorf("config list:\n%s\nwant:\n%s", got, want)
	}
	if _, err := tryRun(app, out, "config", "set", "bad key", "v"); err == nil {
		t.Error("config set accepted a key with a space")
	}
	if _, err := tryRun(app, out, "config", "set", "k", "  "); err == nil {
		t.Error("config set accepted an empty value")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("config set did not write %s: %v", path, err)
	}
}

func TestConfigE2E_SetValidatesSchema(t *testing.T) {
	app, out, _ := e2eApp(t, "")
	app.Config.RegisterSchema(clix.ConfigSchema{Key: "api.retries", Type: clix.ConfigInt})
	app.Config.RegisterSchema(clix.ConfigSchema{Key: "api.debug", Type: clix.ConfigBool})
	if _, err := tryRun(app, out, "config", "set", "api.retries", "many"); err == nil || !strings.Contains(err.Error(), "expected integer") {
		t.Errorf("config set api.retries many: %v", err)
	}
	if got := run(t, app, out, "config", "set", "api.retries", " 5 "); got != "api.retries = 5\n" {
		t.Errorf("config set api.retries 5 printed %q", got)
	}
	if got := run(t, app, out, "config", "set", "api.debug", "TRUE"); got != "api.debug = true\n" {
		t.Errorf("config set api.debug TRUE printed %q", got)
	}
}

// A value saved with "config set" is what a later Run uses for the flag of
// the same name, below the command line and the environment.
func TestConfigE2E_SetValueBecomesFlagDefault(t *testing.T) {
	app, out, _ := e2eApp(t, "")
	var project string
	var source clix.Source
	show := clix.NewCommand("show", clix.WithCommandRun(func(ctx *clix.Context) error {
		f := ctx.App.Flags().Lookup("project")
		source = f.Source()
		_, err := out.WriteString(project + "\n")
		return err
	}))
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project", Usage: "Project"}, Value: &project})
	app.Root.AddCommand(show)
	t.Setenv("PROBE_PROJECT", "")

	run(t, app, out, "config", "set", "project", "from-config")
	if got := run(t, app, out, "show"); got != "from-config\n" || source != clix.SourceConfigFile {
		t.Errorf("show after config set: %q from %v", got, source)
	}
	if got := run(t, app, out, "show", "--project", "from-flag"); got != "from-flag\n" {
		t.Errorf("show --project: %q", got)
	}
	t.Setenv("PROBE_PROJECT", "from-env")
	if got := run(t, app, out, "show"); got != "from-env\n" || source != clix.SourceEnvVar {
		t.Errorf("show with PROBE_PROJECT: %q from %v", got, source)
	}
}

// Without App.ConfigPath the file is config.yaml in $XDG_CONFIG_HOME/<name>.
func TestConfigE2E_DefaultPath(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	out := &bytes.Buffer{}
	app := clix.NewApp("probe", clix.WithAppOut(out))
	app.AddExtension(Extension{})
	run(t, app, out, "config", "set", "a", "1")
	data, err := os.ReadFile(filepath.Join(xdg, "probe", "config.yaml"))
	if err != nil || string(data) != "a: 1\n" {
		t.Errorf("default config file: %q, %v", data, err)
	}
}

func TestConfigE2E_ConfigPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "elsewhere", "my.yaml")
	app, out, _ := e2eApp(t, path)
	run(t, app, out, "config", "set", "a", "1")
	if data, err := os.ReadFile(path); err != nil || string(data) != "a: 1\n" {
		t.Errorf("App.ConfigPath file: %q, %v", data, err)
	}
}

// Config files may hold credentials: the file is 0600 and a directory the
// extension creates is 0700.
func TestConfigE2E_FileModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file modes")
	}
	path := filepath.Join(t.TempDir(), "new", "config.yaml")
	app, out, _ := e2eApp(t, path)
	run(t, app, out, "config", "set", "token", "secret")
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("config file mode %v (%v), want 0600", fi.Mode().Perm(), err)
	}
	if fi, err := os.Stat(filepath.Dir(path)); err != nil || fi.Mode().Perm() != 0o700 {
		t.Errorf("config directory mode %v (%v), want 0700", fi.Mode().Perm(), err)
	}
	// A world-readable file written by hand is tightened on the next save.
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, app, out, "config", "set", "token", "other")
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Errorf("config file mode after a save %v, want 0600", fi.Mode().Perm())
	}
}

// What "config set" does to a hand-written file (see
// clix.ConfigManager.Save): the file is rewritten as a flat, key-sorted
// mapping of dotted keys. Every value it did not change keeps its YAML form,
// so numbers stay numbers and lists stay lists. Comments, blank lines,
// nesting, anchors and merge keys are not kept, and null values are dropped.
func TestConfigE2E_SetRewritesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	orig := "# team defaults\napi:\n  timeout: 30   # seconds\nregions:\n  - eu\n  - us\nbase: &b\n  x: 1\nderived:\n  <<: *b\n  y: yes\nempty:\n"
	if err := os.WriteFile(path, []byte(orig), 0o600); err != nil {
		t.Fatal(err)
	}
	app, out, _ := e2eApp(t, path)
	run(t, app, out, "config", "set", "project", "p1")
	want := "api.timeout: 30\nbase.x: 1\nderived.x: 1\nderived.y: yes\nproject: p1\nregions:\n  - eu\n  - us\n"
	if data, _ := os.ReadFile(path); string(data) != want {
		t.Errorf("file after config set:\n%s\nwant:\n%s", data, want)
	}
}

// Extension commands do not demand the app's required flags.
func TestConfigE2E_SkipsAppRequiredFlags(t *testing.T) {
	app, out, _ := e2eApp(t, "")
	var project string
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project", Required: true}, Value: &project})
	t.Setenv("PROBE_PROJECT", "")
	run(t, app, out, "config", "set", "a", "1")
	if got := run(t, app, out, "config", "list"); got != "a: \"1\"\n" {
		t.Errorf("config list with a required root flag: %q", got)
	}
}

func TestConfigE2E_AddedTwice(t *testing.T) {
	app, out, _ := e2eApp(t, "")
	app.AddExtension(Extension{})
	run(t, app, out, "config", "set", "a", "1")
	if err := app.Validate(); err != nil {
		t.Errorf("App.Validate() = %v", err)
	}
	n := 0
	for _, c := range app.Root.Children {
		if c.Name == "config" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d config commands", n)
	}
}

func TestConfigE2E_SequentialRuns(t *testing.T) {
	app, out, _ := e2eApp(t, "")
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		out.Reset()
		if err := app.Run(ctx, []string{"config", "set", "name", "v"}); err != nil {
			t.Fatal(err)
		}
		if got := run(t, app, out, "config", "get", "name"); got != "v\n" {
			t.Fatalf("run %d: get printed %q", i, got)
		}
		if got := run(t, app, out, "config", "list", "-f", "json"); got != "{\n  \"name\": \"v\"\n}\n" {
			t.Fatalf("run %d: list -f json printed %q", i, got)
		}
		// A later list without -f is YAML again: --format does not leak.
		if got := run(t, app, out, "config", "list"); got != "name: v\n" {
			t.Fatalf("run %d: list printed %q", i, got)
		}
	}
}

// The key and value arguments come only from the command line: an
// environment variable such as PROBE_KEY (often an API key) must neither
// become the key nor appear in an error.
func TestConfigE2E_ArgumentsIgnoreEnvironment(t *testing.T) {
	app, out, _ := e2eApp(t, "")
	run(t, app, out, "config", "set", "a", "1")
	t.Setenv("PROBE_KEY", "s3cret")
	t.Setenv("PROBE_VALUE", "s3cret")
	for _, args := range [][]string{{"config", "get"}, {"config", "set", "b"}, {"config", "unset"}} {
		got, err := tryRun(app, out, args...)
		if err == nil || strings.Contains(err.Error(), "s3cret") || strings.Contains(got, "s3cret") || clix.ExitCode(err) != 2 {
			t.Errorf("%q with PROBE_KEY and PROBE_VALUE set: out %q, err %v", args, got, err)
		}
	}
	if got := run(t, app, out, "config", "get", "a"); got != "1\n" {
		t.Errorf("config get a printed %q", got)
	}
}
