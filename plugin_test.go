//go:build darwin || linux

package clix_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/config"
	"github.com/SCKelemen/clix/v2/ext/help"
)

// writePlugin writes an executable shell script named name into dir.
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

// pluginApp returns an app with plugins on, PATH set to dirs, and its
// output captured.
func pluginApp(t *testing.T, dirs ...string) (*clix.App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PATH", strings.Join(append(dirs, "/usr/bin", "/bin"), string(os.PathListSeparator)))
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	app := clix.NewApp("myapp", clix.WithAppOut(out), clix.WithAppErr(errOut), clix.WithAppInteractive(clix.InteractiveNever))
	app.Plugins = true
	app.Root.AddCommand(clix.NewCommand("deploy", clix.WithCommandAliases("ship"), clix.WithCommandRun(noop)))
	app.Root.AddCommand(clix.NewCommand("secret", clix.WithCommandHidden(true), clix.WithCommandRun(noop)))
	return app, out, errOut
}

// Plugins are found only in absolute PATH directories, first one wins,
// and only executable regular files count.
func TestPluginDiscovery(t *testing.T) {
	base := t.TempDir()
	a, b := filepath.Join(base, "a"), filepath.Join(base, "b")
	first := writePlugin(t, a, "myapp-foo", "exit 0", 0o755)
	second := writePlugin(t, b, "myapp-foo", "exit 0", 0o755)
	writePlugin(t, b, "myapp-bar", "exit 0", 0o755)
	writePlugin(t, b, "myapp-noexec", "exit 0", 0o644)
	writePlugin(t, b, "myapp-bad.name", "exit 0", 0o755)
	writePlugin(t, b, "otherapp-baz", "exit 0", 0o755)
	if err := os.Mkdir(filepath.Join(b, "myapp-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(first, filepath.Join(b, "myapp-linked")); err != nil {
		t.Fatal(err)
	}
	insecure := writePlugin(t, b, "myapp-open", "exit 0", 0o777)
	if err := os.Chmod(insecure, 0o777); err != nil { // umask may have dropped o+w
		t.Fatal(err)
	}
	// The current directory and relative PATH entries are never searched.
	cwd := t.TempDir()
	t.Chdir(cwd)
	writePlugin(t, cwd, "myapp-here", "exit 0", 0o755)
	writePlugin(t, filepath.Join(cwd, "rel"), "myapp-rel", "exit 0", 0o755)

	app, _, _ := pluginApp(t, "", ".", "rel", a, b, a)
	got := app.ListPlugins()
	var names []string
	for _, p := range got {
		names = append(names, p.Name)
	}
	if !reflect.DeepEqual(names, []string{"bar", "foo", "linked", "open"}) {
		t.Fatalf("ListPlugins names = %q", names)
	}
	foo, ok := app.LookupPlugin("foo")
	if !ok || foo.Path != first || !reflect.DeepEqual(foo.Shadowed, []string{second}) || !foo.Runnable() {
		t.Fatalf("LookupPlugin(foo) = %+v", foo)
	}
	if p, _ := app.LookupPlugin("open"); p.Insecure == "" || p.Runnable() {
		t.Fatalf("world-writable plugin: %+v", p)
	}
	for _, name := range []string{"here", "rel", "noexec", "dir", "bad.name", "../a/myapp-foo", "a/b", "", "-x"} {
		if p, ok := app.LookupPlugin(name); ok {
			t.Errorf("LookupPlugin(%q) = %+v", name, p)
		}
	}
	if err := app.Run(bg, []string{"here"}); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("myapp here (only in the current directory): %v", err)
	}
}

func TestPluginRuns(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "myapp-echo", `for a in "$@"; do printf '[%s]\n' "$a"; done
printf 'in=%s\n' "$(cat)"
printf 'plugin=%s name=%s app=%s version=%s path=%s\n' "$MYAPP_PLUGIN" "$MYAPP_PLUGIN_NAME" "$MYAPP_APP_NAME" "$MYAPP_APP_VERSION" "${MYAPP_APP_PATH:+set}"
env | grep -c s3cret
echo oops >&2
exit 5`, 0o755)
	app, out, errOut := pluginApp(t, dir)
	app.Version = "1.2.3"
	app.In = strings.NewReader("piped input")
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "token", Sensitive: true}})
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configFile, []byte("token: s3cret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.ConfigPath = configFile
	err := app.Run(bg, []string{"echo", "a b", "--flag=x", "$(touch pwned)", "", "-"})
	// The plugin's status is Run's.
	var exit *clix.PluginExitError
	if !errors.As(err, &exit) || exit.Code != 5 || clix.ExitCode(err) != 5 || exit.Name != "echo" {
		t.Fatalf("err = %v (%T)", err, err)
	}
	want := "[a b]\n[--flag=x]\n[$(touch pwned)]\n[]\n[-]\nin=piped input\nplugin=1 name=echo app=myapp version=1.2.3 path=set\n0\n"
	if out.String() != want || errOut.String() != "oops\n" {
		t.Fatalf("stdout:\n%s\nwant:\n%s\nstderr %q", out.String(), want, errOut.String())
	}
	if _, err := os.Stat("pwned"); err == nil {
		t.Fatal("an argument ran as shell code")
	}
}

// Plugins never shadow commands, and are looked up only for a first word
// the root would reject.
func TestPluginPrecedence(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(t.TempDir(), "ran")
	for _, name := range []string{"deploy", "ship", "secret", "help", "config", "Deploy", "foo"} {
		writePlugin(t, dir, "myapp-"+name, "echo "+name+" >> "+marker, 0o755)
	}
	app, _, _ := pluginApp(t, dir)
	app.AddExtension(help.Extension{})
	app.AddExtension(config.Extension{})
	for _, args := range [][]string{{"deploy"}, {"ship"}, {"secret"}, {"help"}, {"config"}, {"Deploy"}, {"DEPLOY"}} {
		if err := app.Run(bg, args); err != nil {
			t.Errorf("%q: %v", args, err)
		}
	}
	if data, err := os.ReadFile(marker); err == nil {
		t.Fatalf("plugins ran instead of commands: %q", data)
	}
	for _, args := range [][]string{{"--verbose", "foo"}, {"deploy", "foo"}, {"--", "foo"}} {
		_ = app.Run(bg, args)
	}
	if data, err := os.ReadFile(marker); err == nil {
		t.Fatalf("plugin ran for a word that is not the first: %q", data)
	}
	app.Plugins = false
	if err := app.Run(bg, []string{"foo"}); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("Plugins off: %v", err)
	}
	app.Plugins = true
	if err := app.Run(bg, []string{"foo"}); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(marker); string(data) != "foo\n" {
		t.Fatalf("marker = %q", data)
	}

	// A root that takes positional arguments keeps its word.
	app, _, _ = pluginApp(t, dir)
	var file string
	app.Root.Run = noop
	app.Root.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "file", Positional: true}, Value: &file})
	if err := app.Run(bg, []string{"foo"}); err != nil || file != "foo" {
		t.Fatalf("root positional: %v, file %q", err, file)
	}
}

// A plugin file that every user can write to is never run.
func TestPluginInsecureNotRun(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(t.TempDir(), "ran")
	path := writePlugin(t, dir, "myapp-open", "touch "+marker, 0o777)
	if err := os.Chmod(path, 0o777); err != nil {
		t.Fatal(err)
	}
	app, _, _ := pluginApp(t, dir)
	err := app.Run(bg, []string{"open"})
	if err == nil || !strings.Contains(err.Error(), "writable by every user") || clix.ExitCode(err) != 1 {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the insecure plugin ran")
	}
}

// Main exits with the plugin's status and prints nothing for it.
func TestPluginExitStatusThroughMain(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "myapp-fail", "echo failing >&2; exit 3", 0o755)
	writePlugin(t, dir, "myapp-killed", "kill -KILL $$", 0o755)
	writePlugin(t, dir, "myapp-ok", "exit 0", 0o755)
	app, _, errOut := pluginApp(t, dir)
	if code := clix.MainForTest(app, []string{"fail"}); code != 3 || errOut.String() != "failing\n" {
		t.Fatalf("fail: exit %d, stderr %q", code, errOut.String())
	}
	errOut.Reset()
	if code := clix.MainForTest(app, []string{"killed"}); code != 128+9 {
		t.Fatalf("killed: exit %d", code)
	}
	if code := clix.MainForTest(app, []string{"ok"}); code != 0 {
		t.Fatalf("ok: exit %d", code)
	}
	if code := clix.MainForTest(app, []string{"nosuch"}); code != 2 || !strings.Contains(errOut.String(), "unknown command") {
		t.Fatalf("nosuch: exit %d, stderr %q", code, errOut.String())
	}
}

// While a plugin runs, a SIGTERM to the app is forwarded to it, and a
// SIGINT the app gets does not end the app: Ctrl-C goes to the plugin
// itself through the terminal.
func TestPluginSignals(t *testing.T) {
	dir := t.TempDir()
	state := t.TempDir()
	writePlugin(t, dir, "myapp-wait", `trap 'echo got-term; exit 7' TERM
touch `+filepath.Join(state, "ready")+`
i=0
while [ ! -e `+filepath.Join(state, "go")+` ] && [ $i -lt 500 ]; do sleep 0.01; i=$((i+1)); done
echo done`, 0o755)
	app, out, _ := pluginApp(t, dir)
	waitFile := func(name string) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for {
			if _, err := os.Stat(filepath.Join(state, name)); err == nil {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", name)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	done := make(chan error, 1)
	go func() { done <- app.Run(bg, []string{"wait"}) }()
	waitFile("ready")
	time.Sleep(50 * time.Millisecond) // the trap is set before ready is touched; let Notify settle
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	err := <-done
	var exit *clix.PluginExitError
	if !errors.As(err, &exit) || exit.Code != 7 || !strings.Contains(out.String(), "got-term") {
		t.Fatalf("SIGTERM: err %v, output %q", err, out.String())
	}

	if err := os.Remove(filepath.Join(state, "ready")); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	go func() { done <- app.Run(bg, []string{"wait"}) }()
	waitFile("ready")
	time.Sleep(50 * time.Millisecond)
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(state, "go"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil || out.String() != "done\n" {
		t.Fatalf("SIGINT: err %v, output %q", err, out.String())
	}
}
