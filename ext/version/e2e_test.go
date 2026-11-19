package version

import (
	"bytes"
	"context"
	"encoding/json"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
	"go.yaml.in/yaml/v3"
)

func e2eApp(t *testing.T, ext Extension, opts ...clix.AppOption) (*clix.App, *bytes.Buffer) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out := &bytes.Buffer{}
	opts = append([]clix.AppOption{clix.WithAppOut(out), clix.WithAppErr(&bytes.Buffer{}), clix.WithAppInteractive(clix.InteractiveNever)}, opts...)
	app := clix.NewApp("probe", opts...)
	app.AddExtension(format.Extension{})
	app.AddExtension(ext)
	return app, out
}

func run(t *testing.T, app *clix.App, out *bytes.Buffer, args ...string) string {
	t.Helper()
	out.Reset()
	if err := app.Run(context.Background(), args); err != nil {
		t.Fatalf("%q: %v", args, err)
	}
	return out.String()
}

// fakeBuildInfo makes readBuildInfo return bi for the rest of the test.
func fakeBuildInfo(t *testing.T, bi *debug.BuildInfo) {
	t.Helper()
	saved := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) { return bi, bi != nil }
	t.Cleanup(func() { readBuildInfo = saved })
}

func goLines() string {
	return "go.arch = " + runtime.GOARCH + "\ngo.os = " + runtime.GOOS + "\ngo.version = " + runtime.Version() + "\n"
}

func TestVersionE2E_Text(t *testing.T) {
	app, out := e2eApp(t, Extension{Version: "1.2.3", Commit: "abc123", Date: "2024-01-01"})
	want := "commit = abc123\ndate = 2024-01-01\n" + goLines() + "name = probe\nversion = 1.2.3\n"
	if got := run(t, app, out, "version"); got != want {
		t.Errorf("version:\n%s\nwant:\n%s", got, want)
	}
}

func TestVersionE2E_JSONAndYAML(t *testing.T) {
	app, out := e2eApp(t, Extension{Version: "1.2.3", Commit: "abc123"})
	var j map[string]any
	if err := json.Unmarshal([]byte(run(t, app, out, "version", "--format", "json")), &j); err != nil {
		t.Fatal(err)
	}
	if j["version"] != "1.2.3" || j["commit"] != "abc123" || j["name"] != "probe" || j["go"].(map[string]any)["os"] != runtime.GOOS {
		t.Errorf("version --format json: %v", j)
	}
	if _, ok := j["date"]; ok {
		t.Errorf("an empty Date should be omitted: %v", j)
	}
	var y map[string]any
	if err := yaml.Unmarshal([]byte(run(t, app, out, "-f", "yaml", "version")), &y); err != nil {
		t.Fatal(err)
	}
	if y["version"] != "1.2.3" || y["commit"] != "abc123" {
		t.Errorf("version --format yaml: %v", y)
	}
}

func TestVersionE2E_CommitAndDateOmitted(t *testing.T) {
	app, out := e2eApp(t, Extension{Version: "1.2.3"})
	want := goLines() + "name = probe\nversion = 1.2.3\n"
	if got := run(t, app, out, "version"); got != want {
		t.Errorf("version without Commit and Date:\n%s\nwant:\n%s", got, want)
	}
}

func TestVersionE2E_ShortForm(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"-v"}, {"--version", "version"}} {
		app, out := e2eApp(t, Extension{Version: "1.2.3", Commit: "abc123", Date: "2024-01-01"})
		if got := run(t, app, out, args...); got != "probe version 1.2.3\n" {
			t.Errorf("%q printed %q", args, got)
		}
	}
}

func TestVersionE2E_Help(t *testing.T) {
	app, out := e2eApp(t, Extension{Version: "1.2.3"})
	got := run(t, app, out, "version", "--help")
	if !strings.HasPrefix(got, "Show version information\n") || !strings.Contains(got, "probe version") || strings.Contains(got, "1.2.3") {
		t.Errorf("version --help:\n%s", got)
	}
}

// With Version empty, "version" and "--version" must agree.
func TestVersionE2E_EmptyVersion(t *testing.T) {
	for _, tc := range []struct {
		name  string
		bi    *debug.BuildInfo
		opts  []clix.AppOption
		short string
		text  string
	}{
		{name: "no build info", short: "probe version dev\n", text: "version = dev\n"},
		{name: "devel build without VCS", bi: &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, short: "probe version dev\n", text: "version = dev\n"},
		{name: "go install @version", bi: &debug.BuildInfo{Main: debug.Module{Path: "example.com/probe", Version: "v1.4.0"}}, short: "probe version v1.4.0\n", text: "version = v1.4.0\n"},
		{
			name: "VCS stamp, dirty",
			bi: &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}, Settings: []debug.BuildSetting{
				{Key: "vcs", Value: "git"}, {Key: "vcs.revision", Value: "0123456789abcdef"}, {Key: "vcs.time", Value: "2026-11-19T08:00:00Z"}, {Key: "vcs.modified", Value: "true"},
			}},
			short: "probe version dev\n",
			text:  "commit = 0123456789abcdef\ndate = 2026-11-19T08:00:00Z\n" + goLines() + "modified = true\nname = probe\nversion = dev\n",
		},
		{
			name: "VCS stamp, clean, with module version",
			bi: &debug.BuildInfo{Main: debug.Module{Version: "v1.4.1-0.20261119080000-0123456789ab"}, Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "0123456789abcdef"}, {Key: "vcs.modified", Value: "false"},
			}},
			short: "probe version v1.4.1-0.20261119080000-0123456789ab\n",
			text:  "commit = 0123456789abcdef\n" + goLines() + "name = probe\nversion = v1.4.1-0.20261119080000-0123456789ab\n",
		},
		{name: "App.Version wins over build info", bi: &debug.BuildInfo{Main: debug.Module{Version: "v1.4.0"}}, opts: []clix.AppOption{clix.WithAppVersion("2.0.0")}, short: "probe version 2.0.0\n", text: "version = 2.0.0\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeBuildInfo(t, tc.bi)
			app, out := e2eApp(t, Extension{}, tc.opts...)
			if got := run(t, app, out, "--version"); got != tc.short {
				t.Errorf("--version printed %q, want %q", got, tc.short)
			}
			got := run(t, app, out, "version")
			if strings.Contains(tc.text, "go.version") {
				if got != tc.text {
					t.Errorf("version:\n%s\nwant:\n%s", got, tc.text)
				}
			} else if !strings.HasSuffix(got, tc.text) || strings.Contains(got, "commit") {
				t.Errorf("version:\n%s\nwant it to end with %q", got, tc.text)
			}
		})
	}
}

// Commit and Date given explicitly are not replaced by VCS data, and VCS
// data is not used at all when Version is set.
func TestVersionE2E_ExplicitFieldsWin(t *testing.T) {
	fakeBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Version: "v9.9.9"}, Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "fff"}, {Key: "vcs.time", Value: "2026-01-01T00:00:00Z"}}})
	app, out := e2eApp(t, Extension{Commit: "abc"})
	if got := run(t, app, out, "version"); !strings.HasPrefix(got, "commit = abc\ndate = 2026-01-01T00:00:00Z\n") || !strings.HasSuffix(got, "version = v9.9.9\n") {
		t.Errorf("version with Commit set:\n%s", got)
	}
	app, out = e2eApp(t, Extension{Version: "1.0.0"})
	if got := run(t, app, out, "version"); strings.Contains(got, "fff") || strings.Contains(got, "date") {
		t.Errorf("version with Version set used VCS data:\n%s", got)
	}
}

// The app's own -v always wins; --version keeps working.
func TestVersionE2E_AppShortFlagWins(t *testing.T) {
	addVerbose := func(app *clix.App, verbose *bool) {
		app.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "verbose", Short: "v", Usage: "Verbose output"}, Value: verbose})
	}
	for _, tc := range []struct {
		name    string
		pending bool
		setup   func(t *testing.T, app *clix.App, verbose *bool)
	}{
		{"app flag before AddExtension", false, func(t *testing.T, app *clix.App, verbose *bool) {
			addVerbose(app, verbose)
			app.AddExtension(Extension{Version: "1.0.0"})
		}},
		{"app flag after AddExtension", false, func(t *testing.T, app *clix.App, verbose *bool) {
			app.AddExtension(Extension{Version: "1.0.0"})
			addVerbose(app, verbose)
		}},
		{"app flag after a first Run", true, func(t *testing.T, app *clix.App, verbose *bool) {
			app.AddExtension(Extension{Version: "1.0.0"})
			if err := app.Run(context.Background(), []string{"--version"}); err != nil {
				t.Fatal(err)
			}
			addVerbose(app, verbose)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.pending {
				t.Skip("pending core precedence rule (#36)")
			}
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			out := &bytes.Buffer{}
			app := clix.NewApp("probe", clix.WithAppOut(out))
			var verbose bool
			app.Root.Run = func(*clix.Context) error { return nil }
			tc.setup(t, app, &verbose)
			if got := run(t, app, out, "-v"); got != "" || !verbose {
				t.Errorf("-v printed %q, verbose = %v; want the app's --verbose", got, verbose)
			}
			if got := run(t, app, out, "--version"); got != "probe version 1.0.0\n" {
				t.Errorf("--version printed %q", got)
			}
			if err := app.Validate(); err == nil || !strings.Contains(err.Error(), "-v") {
				t.Errorf("App.Validate() = %v, want a report that --version lost -v", err)
			}
		})
	}
}

func TestVersionE2E_AddedTwice(t *testing.T) {
	app, out := e2eApp(t, Extension{Version: "1.0.0"})
	app.AddExtension(Extension{Version: "2.0.0"})
	if got := run(t, app, out, "-v"); got != "probe version 1.0.0\n" {
		t.Errorf("-v printed %q; the first extension should win", got)
	}
	if err := app.Validate(); err != nil {
		t.Errorf("App.Validate() = %v", err)
	}
	n := 0
	for _, c := range app.Root.Children {
		if c.Name == "version" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d version commands", n)
	}
}

// An app command named "version" replaces the extension's command;
// --version still prints the short form.
func TestVersionE2E_AppVersionCommandWins(t *testing.T) {
	app, out := e2eApp(t, Extension{Version: "1.0.0"})
	ran := false
	app.Root.AddCommand(clix.NewCommand("version", clix.WithCommandRun(func(*clix.Context) error { ran = true; return nil })))
	if got := run(t, app, out, "version"); !ran || got != "" {
		t.Errorf("version ran the app's command = %v, printed %q", ran, got)
	}
	if got := run(t, app, out, "--version"); got != "probe version 1.0.0\n" {
		t.Errorf("--version printed %q", got)
	}
}
