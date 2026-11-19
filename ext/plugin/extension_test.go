//go:build darwin || linux

package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/internal/ptytest"
)

func writePlugin(t *testing.T, dir, name, script string, mode os.FileMode) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

// testApp returns the probe app with PATH set to dirs and captured output.
func testApp(t *testing.T, dirs ...string) (*clix.App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PATH", strings.Join(append(dirs, "/usr/bin", "/bin"), string(os.PathListSeparator)))
	app := newProbeApp()
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	app.Out, app.Err = out, errOut
	app.Interactive = clix.InteractiveNever
	return app, out, errOut
}

func TestPluginList(t *testing.T) {
	base := t.TempDir()
	a, b := filepath.Join(base, "a"), filepath.Join(base, "b")
	foo := writePlugin(t, a, "probe-foo", "exit 0", 0o755)
	shadowed := writePlugin(t, b, "probe-foo", "exit 0", 0o755)
	bar := writePlugin(t, b, "probe-bar", "exit 0", 0o755)
	deploy := writePlugin(t, b, "probe-deploy", "exit 0", 0o755)
	open := writePlugin(t, b, "probe-open", "exit 0", 0o777)
	if err := os.Chmod(open, 0o777); err != nil {
		t.Fatal(err)
	}
	app, out, errOut := testApp(t, a, b)
	if err := app.Run(context.Background(), []string{"plugin", "list"}); err != nil {
		t.Fatal(err)
	}
	want := "NAME    PATH\nbar     " + bar + "\ndeploy  " + deploy + "\nfoo     " + foo + "\nopen    " + open + "\n"
	if out.String() != want {
		t.Fatalf("list:\n%s\nwant:\n%s", out.String(), want)
	}
	for _, w := range []string{
		"warning: " + shadowed + " is shadowed by " + foo + ", earlier on PATH",
		"warning: " + deploy + ` never runs: "deploy" is a command of probe`,
		"warning: " + open + " is not run: it is writable by every user",
	} {
		if !strings.Contains(errOut.String(), w) {
			t.Errorf("stderr lacks %q:\n%s", w, errOut.String())
		}
	}

	out.Reset()
	if err := app.Run(context.Background(), []string{"plugin", "list", "--format", "json"}); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil || len(rows) != 4 || rows[2]["name"] != "foo" || rows[2]["runnable"] != true || rows[1]["runnable"] != false {
		t.Fatalf("json: %v\n%s", err, out.String())
	}

	app, out, errOut = testApp(t, t.TempDir())
	if err := app.Run(context.Background(), []string{"plugin", "list"}); err != nil || out.String() != "" || !strings.Contains(errOut.String(), "no plugins found on PATH (executables named probe-<name>)") {
		t.Fatalf("no plugins: %v, %q, %q", err, out.String(), errOut.String())
	}
	if err := app.Validate(); err != nil {
		t.Fatal(err)
	}
}

// Shell completion offers the plugins that run as the first word, and
// file names after one.
func TestPluginCompletion(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "probe-dance", "exit 0", 0o755)
	writePlugin(t, dir, "probe-deploy", "exit 0", 0o755)
	app, out, _ := testApp(t, dir)
	complete := func(cur string, words ...string) []string {
		t.Helper()
		out.Reset()
		if err := app.Run(context.Background(), append([]string{"__complete", "--cur=" + cur, "--", "probe"}, words...)); err != nil {
			t.Fatal(err)
		}
		return strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	}
	if got := complete("d"); strings.Join(got, "|") != "deploy\tDeploy|dance\tPlugin probe-dance|:nofiles" {
		t.Errorf("d<TAB>: %q", got)
	}
	if got := complete("", "dance"); strings.Join(got, "|") != ":files" {
		t.Errorf("dance <TAB>: %q", got)
	}
	if got := complete("d", "--format", "json"); strings.Contains(strings.Join(got, "|"), "dance") {
		t.Errorf("--format json d<TAB> offers the plugin: %q", got)
	}
	app.Plugins = false
	if got := complete("d"); strings.Contains(strings.Join(got, "|"), "dance") {
		t.Errorf("plugins off: %q", got)
	}
}

// On a real terminal, the plugin gets the terminal, and Ctrl-C reaches it
// while the app waits: the app's exit status is the plugin's.
func TestPluginOnTerminal(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writePlugin(t, dir, "probe-term", `trap 'echo plugin-got-int; exit 42' INT
if [ -t 0 ] && [ -t 1 ]; then echo TTY-yes; else echo TTY-no; fi
echo ready
i=0; while [ $i -lt 1000 ]; do sleep 0.01; i=$((i+1)); done
echo timeout; exit 1`, 0o755)
	term := ptytest.New(t)
	cmd := exec.Command(exe, "term")
	cmd.Env = append(os.Environ(), appEnv+"=1", "PATH="+dir+":/usr/bin:/bin", "XDG_CONFIG_HOME="+t.TempDir())
	cmd.Stdin, cmd.Stdout, cmd.Stderr = term.TTY, term.TTY, term.TTY
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	term.WaitOutput(t, "ready", 20*time.Second)
	if out := term.Output(); !strings.Contains(out, "TTY-yes") {
		t.Fatalf("the plugin did not get the terminal: %q", out)
	}
	term.Type(t, "\x03") // Ctrl-C: SIGINT to the foreground process group
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 42 {
			t.Fatalf("app exited with %v, want status 42; output %q", err, term.Output())
		}
	case <-time.After(20 * time.Second):
		t.Fatalf("the app did not exit; output %q", term.Output())
	}
	time.Sleep(50 * time.Millisecond)
	out := term.Output()
	if !strings.Contains(out, "plugin-got-int") || strings.Contains(out, "exited with status") {
		t.Fatalf("output %q", out)
	}
}
