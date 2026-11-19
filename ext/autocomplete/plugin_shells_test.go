//go:build darwin || linux

package autocomplete

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pluginEnv returns a PATH with the probe app and a probe-zzplug plugin
// (see clix.App.Plugins).
func pluginEnv(t *testing.T) []string {
	t.Helper()
	bin, _, _ := shellEnv(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "probe-zzplug"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return []string{"PATH=" + bin + string(os.PathListSeparator) + dir + string(os.PathListSeparator) + os.Getenv("PATH")}
}

func TestComplete_Plugins(t *testing.T) {
	app, out := testApp(t)
	t.Setenv("PATH", strings.TrimPrefix(pluginEnv(t)[0], "PATH="))
	lines, _ := runComplete(t, app, out, "--cur=zz", "--", "probe")
	assertSet(t, "probe zz<TAB>", lines, []string{"zzplug\tPlugin probe-zzplug"})
	_, directive := runComplete(t, app, out, "--cur=", "--", "probe", "zzplug")
	if directive != directiveFiles {
		t.Errorf("probe zzplug <TAB>: directive %s, want files", directive)
	}
}

func TestShell_Bash_Plugins(t *testing.T) {
	for _, key := range []string{"bash32", "bash5"} {
		t.Run(key, func(t *testing.T) {
			bash := needBash(t, key)
			path := writeScript(t, "bash", "probe.bash")
			env := pluginEnv(t)
			assertSet(t, "probe zz", bashComplete(t, bash, path, "probe zz", "", "", env...), []string{"zzplug"})
			s := bashSession(t, bash, "", env...)
			if got := s.insert("probe zzp"); strings.Join(got, " ") != "probe zzplug" {
				t.Errorf("probe zzp<TAB> gave %q", got)
			}
		})
	}
}

func TestShell_Zsh_Plugins(t *testing.T) {
	s := zshSession(t, pluginEnv(t)...)
	if got := s.insert("probe zzp"); strings.Join(got, " ") != "probe zzplug" {
		t.Errorf("probe zzp<TAB> gave %q", got)
	}
}

// An alias and a plugin with the same name complete as the alias, as Run
// expands the alias and never runs that plugin.
func TestComplete_AliasBeatsPlugin(t *testing.T) {
	app, out := testApp(t)
	t.Setenv("XDG_CONFIG_HOME", strings.TrimPrefix(aliasEnv(t)[0], "XDG_CONFIG_HOME="))
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "probe-zz-get"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	lines, _ := runComplete(t, app, out, "--cur=zz-g", "--", "probe")
	assertSet(t, "probe zz-g<TAB>", lines, []string{"zz-get\tAlias for \"widgets get\""})
	lines, _ = runComplete(t, app, out, "--cur=", "--", "probe", "zz-get")
	assertSet(t, "probe zz-get <TAB>", values(lines), []string{"alpha", "beta", "gamma ray"})
}
