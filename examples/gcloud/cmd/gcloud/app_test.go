package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/examples/gcloud/internal/projects"
)

func init() { projects.OperationStep = 0 }

// run runs a fresh app with args in a private config directory, and
// returns what it wrote to stdout and stderr.
func run(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	app := newApp(clix.WithAppIn(strings.NewReader("")), clix.WithAppOut(&out), clix.WithAppErr(&errOut))
	err = app.Run(context.Background(), args)
	return out.String(), errOut.String(), err
}

// isolate gives the test its own config directory and no gcloud variables.
func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, v := range []string{"CLOUDSDK_CORE_PROJECT", "CLOUDSDK_CORE_CONFIGURATION", "CLOUDSDK_CORE_FORMAT", "CLOUDSDK_CORE_DISABLE_PROMPTS"} {
		t.Setenv(v, "")
	}
}

// The example passes App.Validate, as docs/testing.md recommends checking.
func TestAppValidates(t *testing.T) {
	if err := newApp().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestProjectsList(t *testing.T) {
	isolate(t)
	out, _, err := run(t, "projects", "list")
	if err != nil {
		t.Fatal(err)
	}
	// Not a terminal: tab-separated values, no header.
	if !strings.Contains(out, "demo-staging\tDemo (staging)\t104723940013\n") {
		t.Errorf("projects list:\n%s", out)
	}
	out, _, err = run(t, "projects", "list", "--format", "json")
	if err != nil || !strings.Contains(out, `"projectId": "demo-prod"`) {
		t.Errorf("projects list --format json: %v\n%s", err, out)
	}
}

// A named configuration holds the project; --configuration and
// CLOUDSDK_CORE_CONFIGURATION select one, and --project and
// CLOUDSDK_CORE_PROJECT win over its property.
func TestConfigurations(t *testing.T) {
	isolate(t)
	for _, args := range [][]string{
		{"config", "configurations", "create", "staging"},
		{"config", "set", "project", "demo-staging"},
	} {
		if _, _, err := run(t, args...); err != nil {
			t.Fatalf("%q: %v", args, err)
		}
	}
	describe := func(args ...string) string {
		t.Helper()
		out, _, err := run(t, append(args, "projects", "describe", "--format", "json")...)
		if err != nil {
			t.Fatalf("%q: %v", args, err)
		}
		return out
	}
	if out := describe(); !strings.Contains(out, `"projectId": "demo-staging"`) {
		t.Errorf("describe in staging:\n%s", out)
	}
	if out := describe("--project", "demo-prod"); !strings.Contains(out, `"projectId": "demo-prod"`) {
		t.Errorf("--project should win over the configuration:\n%s", out)
	}

	if _, _, err := run(t, "config", "configurations", "activate", "default"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := run(t, "projects", "describe"); clix.ExitCode(err) != 2 {
		t.Errorf("describe with no project in default: %v, want a usage error", err)
	}
	if out := describe("--configuration", "staging"); !strings.Contains(out, `"projectId": "demo-staging"`) {
		t.Errorf("--configuration staging:\n%s", out)
	}
	t.Setenv("CLOUDSDK_CORE_CONFIGURATION", "staging")
	if out := describe(); !strings.Contains(out, `"projectId": "demo-staging"`) {
		t.Errorf("CLOUDSDK_CORE_CONFIGURATION=staging:\n%s", out)
	}
	t.Setenv("CLOUDSDK_CORE_PROJECT", "sandbox-7731")
	if out := describe(); !strings.Contains(out, `"projectId": "sandbox-7731"`) {
		t.Errorf("CLOUDSDK_CORE_PROJECT should win over the configuration:\n%s", out)
	}

	out, _, err := run(t, "config", "configurations", "list")
	if err != nil || !strings.Contains(out, "staging") || !strings.Contains(out, "default") {
		t.Errorf("configurations list: %v\n%s", err, out)
	}
}

// --clear-labels and --remove-labels exclude each other, and one of the
// three label flags is required.
func TestLabelFlagGroup(t *testing.T) {
	isolate(t)
	for _, tc := range []struct {
		args    []string
		want    string // in stdout
		wantErr string // in the error
	}{
		{args: []string{"--update-labels", "env=prod,owner=ana"}, want: "Updated labels of [demo-staging]: env=prod, owner=ana, team=web\n"},
		{args: []string{"--remove-labels", "team"}, want: "Updated labels of [demo-staging]: env=staging\n"},
		{args: []string{"--clear-labels", "--update-labels", "env=dev"}, want: "Updated labels of [demo-staging]: env=dev\n"},
		{args: []string{"--clear-labels", "--remove-labels", "env"}, wantErr: "--clear-labels and --remove-labels cannot be used together"},
		{args: nil, wantErr: "one of --update-labels, --remove-labels or --clear-labels is required"},
	} {
		args := append([]string{"projects", "update", "demo-staging"}, tc.args...)
		out, _, err := run(t, args...)
		if tc.wantErr != "" {
			var group *clix.FlagGroupError
			if !errors.As(err, &group) || !strings.Contains(err.Error(), tc.wantErr) || clix.ExitCode(err) != 2 {
				t.Errorf("%q: got %v, want a *FlagGroupError %q", args, err, tc.wantErr)
			}
			continue
		}
		if err != nil || out != tc.want {
			t.Errorf("%q: got %q, %v; want %q", args, out, err, tc.want)
		}
	}
	out, _, _ := run(t, "projects", "update", "--help")
	if !strings.Contains(out, "[--clear-labels | --remove-labels=REMOVE_LABELS]") {
		t.Errorf("help should show the groups in the usage line:\n%s", out)
	}
}

// --project and --format are Ambient: they do not turn a call into a
// scripted one, so the missing project ID is still prompted for. Any other
// flag does.
func TestAmbientFlagsKeepPrompting(t *testing.T) {
	isolate(t)
	var out, errOut bytes.Buffer
	app := newApp(
		clix.WithAppIn(strings.NewReader("my-new-project\n")),
		clix.WithAppOut(&out), clix.WithAppErr(&errOut),
		clix.WithAppInteractive(clix.InteractiveAlways),
	)
	if err := app.Run(context.Background(), []string{"--project", "demo-prod", "--format", "json", "projects", "create"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"projectId": "my-new-project"`) {
		t.Errorf("stdout:\n%s", out.String())
	}
	// Not a terminal: the prompt, then one plain progress line, on stderr.
	if want := "Creating project my-new-project...done\n"; !strings.HasSuffix(errOut.String(), want) {
		t.Errorf("stderr %q, want it to end with %q", errOut.String(), want)
	}

	app = newApp(
		clix.WithAppIn(strings.NewReader("my-new-project\n")),
		clix.WithAppOut(&out), clix.WithAppErr(&errOut),
		clix.WithAppInteractive(clix.InteractiveAlways),
	)
	err := app.Run(context.Background(), []string{"projects", "create", "--name", "Demo"})
	var missing *clix.MissingFlagsError
	if !errors.As(err, &missing) {
		t.Errorf("with --name given: %v, want a *MissingFlagsError", err)
	}
}

func TestHelpTopics(t *testing.T) {
	isolate(t)
	out, _, err := run(t, "help", "configurations")
	if err != nil || !strings.Contains(out, "gcloud config configurations create staging") {
		t.Errorf("help configurations: %v\n%s", err, out)
	}
	out, _, err = run(t, "help", "environment")
	if err != nil || !strings.Contains(out, "CLOUDSDK_CORE_PROJECT") {
		t.Errorf("help environment: %v\n%s", err, out)
	}
	out, _, _ = run(t, "--help")
	if !strings.Contains(out, "ADDITIONAL HELP TOPICS") || !strings.Contains(out, "configurations") {
		t.Errorf("root help should list the topics:\n%s", out)
	}
}

func TestAuthLogin(t *testing.T) {
	isolate(t)
	out, _, err := run(t, "auth", "login", "ana@example.com", "--brief")
	if err != nil || out != "Logged in as ana@example.com with brief output.\n" {
		t.Errorf("auth login: %q, %v", out, err)
	}
}
