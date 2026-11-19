package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
)

// badValuesApp has the config and format extensions, a root --verbose
// switch, a root --retries int read from the "api.retries" key, and a
// "deploy" command whose --region only accepts eu or us.
func badValuesApp(t *testing.T, path string) (*clix.App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PROBE_VERBOSE", "")
	t.Setenv("PROBE_FORMAT", "")
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	app := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(errOut), clix.WithAppInteractive(clix.InteractiveNever))
	app.ConfigPath = path
	app.AddExtension(Extension{})
	app.AddExtension(format.Extension{})
	var verbose bool
	var retries int
	var region string
	app.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "verbose", Usage: "Verbose"}, Value: &verbose})
	app.Flags().IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "retries", ConfigKey: "api.retries"}, Value: &retries})
	deploy := clix.NewCommand("deploy", clix.WithCommandRun(func(*clix.Context) error { return nil }))
	deploy.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "region", Validate: func(v string) error {
		if v != "eu" && v != "us" {
			return errors.New("must be eu or us")
		}
		return nil
	}}, Value: &region})
	app.Root.AddCommand(deploy)
	return app, out, errOut
}

// "config set" refuses a value that a flag reading the key would reject,
// since every later run would fail on it.
func TestConfigE2E_SetRefusesValuesFlagsReject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	app, out, _ := badValuesApp(t, path)
	for _, tc := range []struct{ key, value, want string }{
		{"format", "xml", `invalid value "xml" for config key "format", which sets --format: unknown format "xml" (valid formats: json, yaml, text)`},
		{"verbose", "notabool", `config key "verbose", which sets --verbose`},
		{"api.retries", "many", `config key "api.retries", which sets --retries`},
		{"region", "mars", `config key "region", which sets --region: must be eu or us`},
	} {
		_, err := tryRun(app, out, "config", "set", tc.key, tc.value)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("config set %s %s: err %v, want it to contain %q", tc.key, tc.value, err, tc.want)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a refused set wrote the file (%v)", err)
	}
	for _, kv := range [][2]string{{"format", "JSON"}, {"verbose", "true"}, {"api.retries", "3"}, {"region", "eu"}} {
		run(t, app, out, "config", "set", kv[0], kv[1])
	}
	run(t, app, out, "deploy")

	// Without AutoConfig, flags read only their ConfigKey, so "format" is
	// a plain key again.
	off := false
	app2, out2, _ := badValuesApp(t, filepath.Join(t.TempDir(), "config.yaml"))
	app2.AutoConfig = &off
	run(t, app2, out2, "config", "set", "format", "xml")
	run(t, app2, out2, "deploy")
	if _, err := tryRun(app2, out2, "config", "set", "api.retries", "many"); err == nil {
		t.Error("ConfigKey flags are still checked without AutoConfig")
	}
}

// A bad value already in the file or the environment makes the app's own
// commands fail, but every config command still runs (with a warning on
// App.Err), so the value can be inspected and fixed.
func TestConfigE2E_CommandsSurviveBadValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("format: xml\nverbose: notabool\nname: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app, out, errOut := badValuesApp(t, path)
	if _, err := tryRun(app, out, "deploy"); err == nil || !strings.Contains(err.Error(), "from config file key") || clix.ExitCode(err) != 2 {
		t.Fatalf("deploy with bad values in the file: %v", err)
	}

	errOut.Reset()
	if got := run(t, app, out, "config", "list"); got != "format: xml\nname: x\nverbose: notabool\n" {
		t.Errorf("config list:\n%s", got)
	}
	if !strings.Contains(errOut.String(), "probe: ignoring invalid value") {
		t.Errorf("no warning on App.Err: %q", errOut.String())
	}
	if got := run(t, app, out, "config", "get", "format"); got != "xml\n" {
		t.Errorf("config get format: %q", got)
	}
	if got := run(t, app, out, "config"); !strings.Contains(got, "Manage CLI configuration") {
		t.Errorf("config:\n%s", got)
	}
	run(t, app, out, "config", "set", "format", "json")
	run(t, app, out, "config", "unset", "verbose")
	errOut.Reset()
	run(t, app, out, "deploy")
	if errOut.Len() != 0 {
		t.Errorf("warnings after the fix: %q", errOut.String())
	}

	// An environment variable the flag rejects.
	t.Setenv("PROBE_VERBOSE", "zzz")
	if _, err := tryRun(app, out, "deploy"); err == nil || !strings.Contains(err.Error(), "PROBE_VERBOSE") {
		t.Errorf("deploy with PROBE_VERBOSE=zzz: %v", err)
	}
	for _, args := range [][]string{{"config", "list"}, {"config", "get", "format"}, {"config", "set", "a", "1"}, {"config", "unset", "a"}} {
		run(t, app, out, args...)
	}
}

func TestConfigE2E_ResetRepairsBadValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("format: xml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app, out, _ := badValuesApp(t, path)
	if _, err := tryRun(app, out, "deploy"); err == nil {
		t.Fatal("deploy ran with format: xml")
	}
	run(t, app, out, "config", "reset")
	run(t, app, out, "deploy")
}

// On a file that is not valid YAML, list, get, set and unset refuse with a
// hint (set must not replace the file with one key); reset still works.
func TestConfigE2E_MalformedFileHint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("a: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app, out, _ := badValuesApp(t, path)
	for _, args := range [][]string{{"config", "list"}, {"config", "get", "a"}, {"config", "set", "a", "1"}, {"config", "unset", "a"}} {
		_, err := tryRun(app, out, args...)
		if err == nil || !strings.Contains(err.Error(), `run "probe config reset" to remove the file`) {
			t.Errorf("%q on a malformed file: %v", args, err)
		}
	}
	if data, _ := os.ReadFile(path); string(data) != "a: [\n" {
		t.Errorf("the malformed file changed:\n%s", data)
	}
	run(t, app, out, "config", "reset")
	if _, err := tryRun(app, out, "config", "list"); err != nil {
		t.Errorf("config list after reset: %v", err)
	}
}
