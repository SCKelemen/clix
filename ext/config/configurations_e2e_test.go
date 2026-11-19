package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/autocomplete"
	"github.com/SCKelemen/clix/v2/ext/format"
)

// cfgApp is an app with named configurations whose config directory is a
// temporary directory: config.yaml (default), configurations/ and
// active_config live in dir.
type cfgApp struct {
	app     *clix.App
	out     *bytes.Buffer
	errOut  *bytes.Buffer
	dir     string
	project string // --project as the "show" command saw it
	source  clix.Source
}

func newCfgApp(t *testing.T, opts ...clix.AppOption) *cfgApp {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PROBE_CONFIGURATION", "")
	c := &cfgApp{out: &bytes.Buffer{}, errOut: &bytes.Buffer{}}
	c.dir = filepath.Join(t.TempDir(), "probe")
	opts = append([]clix.AppOption{clix.WithAppOut(c.out), clix.WithAppErr(c.errOut), clix.WithAppInteractive(clix.InteractiveNever)}, opts...)
	c.app = clix.NewApp("probe", opts...)
	c.app.ConfigPath = filepath.Join(c.dir, "config.yaml")
	c.app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project", Usage: "Project"}})
	c.app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "token", Usage: "API token", Sensitive: true}})
	c.app.Root.AddCommand(clix.NewCommand("show", clix.WithCommandRun(func(ctx *clix.Context) error {
		c.project, c.source, _ = ctx.EffectiveString("project")
		return nil
	})))
	c.app.AddExtension(Extension{Configurations: true})
	c.app.AddExtension(format.Extension{})
	return c
}

func (c *cfgApp) run(t *testing.T, args ...string) string {
	t.Helper()
	out, err := c.try(args...)
	if err != nil {
		t.Fatalf("%q: %v (stderr %q)", args, err, c.errOut.String())
	}
	return out
}

func (c *cfgApp) try(args ...string) (string, error) {
	c.out.Reset()
	c.errOut.Reset()
	c.project = ""
	err := c.app.Run(context.Background(), args)
	return c.out.String(), err
}

func (c *cfgApp) write(t *testing.T, rel, body string) {
	t.Helper()
	path := filepath.Join(c.dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (c *cfgApp) read(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(c.dir, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func mode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

// The classic config file is the default configuration, read in place:
// nothing moves, and creating, activating and setting go to the new
// layout with private file modes.
func TestConfigurationsLayout(t *testing.T) {
	c := newCfgApp(t)
	c.write(t, "config.yaml", "# my settings\nproject: classic\nmask: 0x1F\n")
	before := c.read(t, "config.yaml")

	c.run(t, "show")
	if c.project != "classic" || c.source != clix.SourceConfigFile {
		t.Fatalf("default configuration: project %q (%v)", c.project, c.source)
	}
	if got := c.run(t, "config", "configurations", "list"); got != "NAME     IS_ACTIVE\ndefault  true\n" {
		t.Fatalf("list:\n%s", got)
	}

	if got := c.run(t, "config", "configurations", "create", "work"); got != "Created configuration \"work\"\nActivated configuration \"work\"\n" {
		t.Fatalf("create:\n%s", got)
	}
	if got := c.read(t, "active_config"); got != "work\n" {
		t.Fatalf("active_config = %q", got)
	}
	c.run(t, "config", "set", "project", "p-work")
	if got := c.read(t, "configurations/config_work.yaml"); got != "project: p-work\n" {
		t.Fatalf("config_work.yaml = %q", got)
	}
	if got := c.read(t, "config.yaml"); got != before {
		t.Fatalf("config.yaml changed:\n%s", got)
	}
	if runtime.GOOS != "windows" {
		for path, want := range map[string]os.FileMode{
			filepath.Join(c.dir, "configurations"):                     0o700,
			filepath.Join(c.dir, "configurations", "config_work.yaml"): 0o600,
			filepath.Join(c.dir, "active_config"):                      0o600,
		} {
			if got := mode(t, path); got != want {
				t.Errorf("%s: mode %o, want %o", path, got, want)
			}
		}
	}
	entries, _ := os.ReadDir(c.dir)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("temporary file left behind: %s", e.Name())
		}
	}

	c.run(t, "show")
	if c.project != "p-work" {
		t.Fatalf("work configuration: project %q", c.project)
	}
	if got := c.run(t, "config", "get", "project"); got != "p-work\n" {
		t.Fatalf("config get = %q", got)
	}
	if got := c.run(t, "config", "list"); got != "project: p-work\n" {
		t.Fatalf("config list = %q", got)
	}
	c.run(t, "config", "configurations", "activate", "default")
	c.run(t, "show")
	if c.project != "classic" {
		t.Fatalf("back to default: project %q", c.project)
	}
	if got := c.run(t, "config", "get", "mask"); got != "0x1F\n" {
		t.Fatalf("default keeps its values: %q", got)
	}
	if got := c.run(t, "config", "configurations", "list"); got != "NAME     IS_ACTIVE\ndefault  true\nwork     false\n" {
		t.Fatalf("list:\n%s", got)
	}
}

// --configuration beats <PREFIX>_CONFIGURATION, which beats active_config,
// which beats "default".
func TestConfigurationSelection(t *testing.T) {
	c := newCfgApp(t)
	c.write(t, "config.yaml", "project: p-default\n")
	c.write(t, "configurations/config_work.yaml", "project: p-work\n")
	c.write(t, "configurations/config_home.yaml", "project: p-home\n")

	check := func(want string, wantSource clix.Source, args ...string) {
		t.Helper()
		c.run(t, append(args, "show")...)
		if c.project != "p-"+want {
			t.Errorf("%q: project %q, want p-%s", args, c.project, want)
		}
		name, source, err := c.app.Configuration()
		if err != nil || name != want || source != wantSource {
			t.Errorf("%q: Configuration() = %q, %v, %v; want %q, %v", args, name, source, err, want, wantSource)
		}
	}
	check("default", clix.SourceDefault)
	c.run(t, "config", "configurations", "activate", "work")
	check("work", clix.SourceConfigFile)
	t.Setenv("PROBE_CONFIGURATION", "home")
	check("home", clix.SourceEnvVar)
	check("default", clix.SourceAppFlag, "--configuration", "default")
	check("work", clix.SourceAppFlag, "--configuration=work")
	// The flag is Ambient and global: after the command name too.
	c.run(t, "show", "--configuration", "home")
	if c.project != "p-home" {
		t.Errorf("show --configuration home: project %q", c.project)
	}
	// The command line beats the configuration's own values.
	c.run(t, "--configuration", "work", "show", "--project", "cli")
	if c.project != "cli" {
		t.Errorf("--project cli: %q", c.project)
	}
	// list marks the configuration in effect and says why.
	out := c.run(t, "config", "configurations", "list")
	if out != "NAME     IS_ACTIVE\ndefault  false\nhome     true\nwork     false\n" || !strings.Contains(c.errOut.String(), `$PROBE_CONFIGURATION selects "home"`) {
		t.Errorf("list:\n%s%s", out, c.errOut.String())
	}
	// Activating while the environment overrides it warns.
	c.run(t, "config", "configurations", "activate", "default")
	if !strings.Contains(c.errOut.String(), `warning: $PROBE_CONFIGURATION selects "home"`) {
		t.Errorf("activate under PROBE_CONFIGURATION: stderr %q", c.errOut.String())
	}
	if got := c.read(t, "active_config"); got != "default\n" {
		t.Errorf("active_config = %q", got)
	}
}

// A configuration that does not exist, or a name that is not valid, is an
// error; the configuration commands still run so it can be fixed.
func TestConfigurationSelectionErrors(t *testing.T) {
	c := newCfgApp(t)
	c.write(t, "config.yaml", "project: p-default\n")

	for _, tc := range []struct {
		args []string
		env  string
		code int
		msg  string
	}{
		{[]string{"--configuration", "nope", "show"}, "", 2, `configuration "nope" (from --configuration) does not exist`},
		{[]string{"show"}, "nope", 2, `configuration "nope" (from $PROBE_CONFIGURATION) does not exist`},
		{[]string{"--configuration", "../etc", "show"}, "", 2, `invalid configuration name "../etc"`},
		{[]string{"show"}, "Work", 2, `invalid configuration name "Work"`},
	} {
		t.Setenv("PROBE_CONFIGURATION", tc.env)
		_, err := c.try(tc.args...)
		if err == nil || !strings.Contains(err.Error(), tc.msg) || clix.ExitCode(err) != tc.code {
			t.Errorf("%q with PROBE_CONFIGURATION=%q: %v (exit %d), want %q (exit %d)", tc.args, tc.env, err, clix.ExitCode(err), tc.msg, tc.code)
		}
		if c.project != "" {
			t.Errorf("%q: ran with project %q", tc.args, c.project)
		}
	}

	// The configuration commands keep working, and config get explains.
	t.Setenv("PROBE_CONFIGURATION", "nope")
	out := c.run(t, "config", "configurations", "list")
	if out != "NAME     IS_ACTIVE\ndefault  false\n" || !strings.Contains(c.errOut.String(), `warning: configuration "nope"`) {
		t.Errorf("list:\n%s%s", out, c.errOut.String())
	}
	_, err := c.try("config", "get", "project")
	if err == nil || !errors.Is(err, clix.ErrConfigurationNotFound) || !strings.Contains(err.Error(), `run "probe config configurations list"`) {
		t.Errorf("config get: %v", err)
	}
	if _, err := c.try("config", "set", "project", "x"); err == nil {
		t.Error("config set saved into a configuration that does not exist")
	}
	c.run(t, "config", "configurations", "create", "nope", "--no-activate")
	c.run(t, "show")
	if c.project != "" {
		t.Errorf("new empty configuration: project %q", c.project)
	}
	t.Setenv("PROBE_CONFIGURATION", "")

	// An active_config that names a missing configuration, or holds
	// something that is not a name, is an error until it is fixed.
	for _, pointer := range []string{"gone\n", "../../../etc/passwd\n"} {
		c.write(t, "active_config", pointer)
		_, err := c.try("show")
		if err == nil || clix.ExitCode(err) != 1 || !strings.Contains(err.Error(), "active_config") {
			t.Errorf("active_config %q: %v (exit %d)", pointer, err, clix.ExitCode(err))
		}
		c.run(t, "config", "configurations", "activate", "default")
		c.run(t, "show")
		if c.project != "p-default" {
			t.Errorf("after activate default: %q", c.project)
		}
	}

	// Names that could leave the configurations directory never reach the
	// file system.
	for _, name := range []string{"../x", "a/b", `a\b`, "", "-x", "UPPER", "x.y", strings.Repeat("a", 65)} {
		for _, args := range [][]string{
			{"config", "configurations", "create", name},
			{"config", "configurations", "activate", name},
			{"config", "configurations", "describe", name},
			{"config", "configurations", "delete", "--yes", name},
			{"config", "configurations", "rename", "nope", name},
		} {
			if name == "" && args[2] == "describe" {
				continue // describe "" describes the active configuration
			}
			if _, err := c.try(args...); err == nil {
				t.Errorf("%q succeeded", args)
			}
		}
	}
	entries, _ := os.ReadDir(c.dir)
	for _, e := range entries {
		if n := e.Name(); n != "config.yaml" && n != "configurations" && n != "active_config" {
			t.Errorf("unexpected file %s", n)
		}
	}
}

func TestConfigurationsDeleteAndRename(t *testing.T) {
	c := newCfgApp(t)
	c.run(t, "config", "configurations", "create", "work")
	c.run(t, "config", "configurations", "create", "old", "--no-activate")
	c.run(t, "config", "configurations", "create", "spare", "--no-activate")

	refusals := map[string][]string{
		`cannot delete the "default" configuration; use "probe config reset" to clear it`: {"config", "configurations", "delete", "-y", "default"},
		`cannot delete the active configuration "work"`:                                   {"config", "configurations", "delete", "-y", "work"},
		`cannot delete configuration "old": this command uses it (from --configuration)`:  {"--configuration", "old", "config", "configurations", "delete", "-y", "old"},
		`configuration "nope" does not exist`:                                             {"config", "configurations", "delete", "-y", "spare", "nope"},
		`cannot rename the "default" configuration`:                                       {"config", "configurations", "rename", "default", "x"},
		`cannot rename the active configuration "work"`:                                   {"config", "configurations", "rename", "work", "x"},
		`configuration "spare" already exists`:                                            {"config", "configurations", "rename", "old", "spare"},
	}
	for want, args := range refusals {
		if _, err := c.try(args...); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want %q", args, err, want)
		}
	}
	// Nothing was deleted by the refused "spare nope".
	if names, _ := c.app.Configurations(); !reflect.DeepEqual(names, []string{"default", "old", "spare", "work"}) {
		t.Fatalf("Configurations = %q", names)
	}

	if got := c.run(t, "config", "configurations", "rename", "old", "--new-name", "new"); got != "Renamed configuration \"old\" to \"new\"\n" {
		t.Fatalf("rename: %q", got)
	}
	if got := c.run(t, "config", "configurations", "delete", "spare", "new"); got != "Deleted configuration \"spare\"\nDeleted configuration \"new\"\n" {
		t.Fatalf("delete: %q", got)
	}
	if names, _ := c.app.Configurations(); !reflect.DeepEqual(names, []string{"default", "work"}) {
		t.Fatalf("Configurations = %q", names)
	}
}

// When the app may ask, delete asks first; --yes skips the question.
func TestConfigurationsDeleteConfirmation(t *testing.T) {
	c := newCfgApp(t, clix.WithAppInteractive(clix.InteractiveAlways))
	c.app.Prompter = clix.TextPrompter{In: strings.NewReader("n\ny\n"), Out: c.errOut}
	c.run(t, "config", "configurations", "create", "a", "--no-activate")
	if _, err := c.try("config", "configurations", "delete", "a"); err == nil || err.Error() != "nothing deleted" {
		t.Fatalf("answered no: %v", err)
	}
	if !strings.Contains(c.errOut.String(), `Delete configuration "a"`) {
		t.Fatalf("prompt: %q", c.errOut.String())
	}
	c.run(t, "config", "configurations", "delete", "a")
	c.run(t, "config", "configurations", "create", "b", "--no-activate")
	c.app.Prompter = clix.TextPrompter{In: strings.NewReader(""), Out: c.errOut}
	c.run(t, "config", "configurations", "delete", "--yes", "b")
	if names, _ := c.app.Configurations(); !reflect.DeepEqual(names, []string{"default"}) {
		t.Fatalf("Configurations = %q", names)
	}
}

// describe shows a configuration's values with secrets redacted.
func TestConfigurationsDescribe(t *testing.T) {
	c := newCfgApp(t)
	c.write(t, "configurations/config_work.yaml", "project: p\ntoken: s3cret\napi:\n  timeout: 30s\n")
	c.run(t, "config", "configurations", "activate", "work")
	want := "name: work\nis_active: true\nproperties:\n  api:\n    timeout: 30s\n  project: p\n  token: '[redacted]'\n"
	for _, args := range [][]string{{"config", "configurations", "describe"}, {"config", "configurations", "describe", "work"}} {
		if got := c.run(t, args...); got != want {
			t.Errorf("%q:\n%s\nwant\n%s", args, got, want)
		}
	}
	if got := c.run(t, "config", "configurations", "describe", "default"); got != "name: default\nis_active: false\nproperties: {}\n" {
		t.Errorf("describe default:\n%s", got)
	}
	var desc map[string]any
	out := c.run(t, "--format", "json", "config", "configurations", "describe")
	if err := json.Unmarshal([]byte(out), &desc); err != nil || desc["is_active"] != true || strings.Contains(out, "s3cret") {
		t.Errorf("json: %v\n%s", err, out)
	}
	var rows []map[string]any
	out = c.run(t, "config", "configurations", "list", "--format=json")
	if err := json.Unmarshal([]byte(out), &rows); err != nil || len(rows) != 2 || rows[1]["name"] != "work" || rows[1]["is_active"] != true {
		t.Errorf("list json: %v\n%s", err, out)
	}
	if _, err := c.try("config", "configurations", "describe", "nope"); !errors.Is(err, clix.ErrConfigurationNotFound) {
		t.Errorf("describe nope: %v", err)
	}
}

// config reset empties a named configuration instead of removing it, and
// a malformed file in a named configuration is handled as in config.yaml.
func TestConfigurationsResetAndBrokenFile(t *testing.T) {
	c := newCfgApp(t)
	c.write(t, "config.yaml", "project: keep\n")
	c.run(t, "config", "configurations", "create", "work")
	c.write(t, "configurations/config_work.yaml", "project: [unclosed\n")
	if _, err := c.try("show"); err == nil {
		t.Fatal("a malformed configuration file was ignored")
	}
	if _, err := c.try("config", "list"); err == nil || !strings.Contains(err.Error(), `run "probe config reset"`) {
		t.Fatalf("config list: %v", err)
	}
	c.run(t, "config", "reset")
	if got := c.read(t, "configurations/config_work.yaml"); got != "{}\n" {
		t.Fatalf("after reset: %q", got)
	}
	if names, _ := c.app.Configurations(); !reflect.DeepEqual(names, []string{"default", "work"}) {
		t.Fatalf("reset removed the configuration: %q", names)
	}
	c.run(t, "show")
	if c.project != "" {
		t.Fatalf("project %q after reset", c.project)
	}
	if got := c.read(t, "config.yaml"); got != "project: keep\n" {
		t.Fatalf("config.yaml = %q", got)
	}
}

// Configuration files and active_config may be symbolic links: writes go
// through them, rename moves the link, and delete removes only the link.
func TestConfigurationsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need privileges on Windows")
	}
	c := newCfgApp(t)
	dotfiles := t.TempDir()
	target := filepath.Join(dotfiles, "work.yaml")
	if err := os.WriteFile(target, []byte("project: linked\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pointer := filepath.Join(dotfiles, "active")
	if err := os.MkdirAll(filepath.Join(c.dir, "configurations"), 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(c.dir, "configurations", "config_work.yaml")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(pointer, filepath.Join(c.dir, "active_config")); err != nil {
		t.Fatal(err)
	}
	c.run(t, "config", "configurations", "activate", "work")
	if data, _ := os.ReadFile(pointer); string(data) != "work\n" {
		t.Fatalf("active_config's target = %q", data)
	}
	if info, err := os.Lstat(filepath.Join(c.dir, "active_config")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("active_config is no longer a link")
	}
	c.run(t, "show")
	if c.project != "linked" {
		t.Fatalf("project %q", c.project)
	}
	c.run(t, "config", "set", "project", "changed")
	if data, _ := os.ReadFile(target); string(data) != "project: changed\n" {
		t.Fatalf("target = %q", data)
	}
	c.run(t, "config", "configurations", "activate", "default")
	c.run(t, "config", "configurations", "rename", "work", "job")
	if dest, err := os.Readlink(filepath.Join(c.dir, "configurations", "config_job.yaml")); err != nil || dest != target {
		t.Fatalf("renamed link: %q, %v", dest, err)
	}
	c.run(t, "config", "configurations", "delete", "job")
	if _, err := os.Lstat(filepath.Join(c.dir, "configurations", "config_job.yaml")); !os.IsNotExist(err) {
		t.Fatalf("link not removed: %v", err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "project: changed\n" {
		t.Fatalf("delete removed the link's target: %q, %v", data, err)
	}
}

// Without App.ConfigPath the layout lives in $XDG_CONFIG_HOME/<name>.
func TestConfigurationsDefaultDirectory(t *testing.T) {
	c := newCfgApp(t)
	c.app.ConfigPath = ""
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	c.run(t, "config", "configurations", "create", "work")
	for _, rel := range []string{"probe/configurations/config_work.yaml", "probe/active_config"} {
		if _, err := os.Stat(filepath.Join(xdg, rel)); err != nil {
			t.Errorf("%s: %v", rel, err)
		}
	}
	if dir, err := c.app.ConfigDir(); err != nil || dir != filepath.Join(xdg, "probe") {
		t.Errorf("ConfigDir = %q, %v", dir, err)
	}
	if file, err := c.app.ConfigFile(); err != nil || file != filepath.Join(xdg, "probe", "configurations", "config_work.yaml") {
		t.Errorf("ConfigFile = %q, %v", file, err)
	}
}

// Each Run reads the configuration it selects, and only that one.
func TestConfigurationsRepeatedRuns(t *testing.T) {
	c := newCfgApp(t)
	c.write(t, "config.yaml", "project: p-default\n")
	c.write(t, "configurations/config_work.yaml", "region: eu\n")
	for i := 0; i < 2; i++ {
		c.run(t, "--configuration", "work", "show")
		if c.project != "" {
			t.Fatalf("run %d: default's project leaked into work: %q", i, c.project)
		}
		if got := c.run(t, "--configuration", "work", "config", "list"); got != "region: eu\n" {
			t.Fatalf("run %d: work list = %q", i, got)
		}
		c.run(t, "show")
		if c.project != "p-default" {
			t.Fatalf("run %d: default project %q", i, c.project)
		}
	}
}

// The extension's --configuration is Ambient (it still lets Run prompt)
// and loses to an app flag of the same name, as extension flags do.
func TestConfigurationFlagOwnership(t *testing.T) {
	c := newCfgApp(t, clix.WithAppInteractive(clix.InteractiveAlways))
	c.write(t, "configurations/config_work.yaml", "project: p-work\n")
	var name, project string
	mk := clix.NewCommand("mk", clix.WithCommandRun(func(ctx *clix.Context) error {
		project, _ = ctx.String("project")
		return nil
	}))
	mk.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "name", Required: true}, Value: &name})
	c.app.Root.AddCommand(mk)
	c.app.Prompter = clix.TextPrompter{In: strings.NewReader("w1\n"), Out: c.errOut}
	if err := c.app.Validate(); err != nil {
		t.Fatal(err)
	}
	c.run(t, "--configuration", "work", "mk")
	if name != "w1" || project != "p-work" {
		t.Fatalf("name %q, project %q", name, project)
	}

	// An app flag named --configuration wins; the environment variable
	// still selects.
	app, out, _ := e2eApp(t, "")
	var mine string
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "configuration", NoEnv: true}, Value: &mine})
	app.AddExtension(Extension{Configurations: true})
	if err := app.Validate(); err == nil || !strings.Contains(err.Error(), "the app's --configuration is used") {
		t.Fatalf("Validate: %v", err)
	}
	if _, err := tryRun(app, out, "--configuration", "anything", "config", "list"); err != nil || mine != "anything" {
		t.Fatalf("app flag: %v, %q", err, mine)
	}
	t.Setenv("PROBE_CONFIGURATION", "nope")
	if _, err := tryRun(app, out, "config", "list"); !errors.Is(err, clix.ErrConfigurationNotFound) {
		t.Fatalf("PROBE_CONFIGURATION with the app's flag: %v", err)
	}
}

// Without the Configurations option nothing changes: no flag, no
// commands, and config.yaml is the only file.
func TestConfigurationsOff(t *testing.T) {
	app, out, path := e2eApp(t, "")
	run(t, app, out, "config", "set", "a", "b")
	if app.Flags().Lookup("configuration") != nil || app.ConfigurationsEnabled() {
		t.Fatal("--configuration registered without the option")
	}
	if _, err := tryRun(app, out, "config", "configurations", "list"); err == nil {
		t.Fatal("config configurations exists without the option")
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 || entries[0].Name() != "config.yaml" {
		t.Fatalf("files: %v", entries)
	}
	if err := app.CreateConfiguration("x"); err == nil {
		t.Fatal("CreateConfiguration worked without EnableConfigurations")
	}
	if names, _ := app.Configurations(); !reflect.DeepEqual(names, []string{"default"}) {
		t.Fatalf("Configurations = %q", names)
	}
}

// Shell completion offers the configurations for --configuration and for
// the commands that take an existing one.
func TestConfigurationsCompletion(t *testing.T) {
	c := newCfgApp(t)
	c.app.AddExtension(autocomplete.Extension{})
	c.run(t, "config", "configurations", "create", "work")
	c.run(t, "config", "configurations", "create", "home", "--no-activate")
	for _, words := range [][]string{
		{"--configuration"},
		{"config", "configurations", "activate"},
		{"config", "configurations", "describe"},
		{"config", "configurations", "delete", "home"},
		{"config", "configurations", "rename"},
	} {
		out := c.run(t, append([]string{"__complete", "--cur=", "--", "probe"}, words...)...)
		got := strings.Split(strings.TrimSpace(out), "\n")
		want := []string{"default", "home", "work\tactive", ":nofiles"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q <TAB>: %q, want %q", words, got, want)
		}
	}
	out := c.run(t, "__complete", "--cur=--conf", "--", "probe")
	if !strings.Contains(out, "--configuration\t") {
		t.Errorf("--conf<TAB>: %q", out)
	}
}

func TestConfigurationsValidate(t *testing.T) {
	c := newCfgApp(t)
	c.app.AddExtension(autocomplete.Extension{})
	if err := c.app.Validate(); err != nil {
		t.Fatal(err)
	}
}
