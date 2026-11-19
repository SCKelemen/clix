//go:build darwin || linux

package autocomplete

import (
	"os/exec"
	"strings"
	"testing"
)

func TestShell_Bash_UserAliases(t *testing.T) {
	for _, key := range []string{"bash32", "bash5"} {
		t.Run(key, func(t *testing.T) {
			bash := needBash(t, key)
			path := writeScript(t, "bash", "probe.bash")
			env := aliasEnv(t)
			assertSet(t, "probe zz", bashComplete(t, bash, path, "probe zz", "", "", env...), []string{"zz-get", "zz-out"})
			assertSet(t, "probe zz-get ", bashComplete(t, bash, path, "probe zz-get ", "", "", env...), []string{"alpha", "beta", `gamma\ ray`})
			s := bashSession(t, bash, "", env...)
			if got := s.insert("probe zz-g"); strings.Join(got, " ") != "probe zz-get" {
				t.Errorf("probe zz-g<TAB> gave %q", got)
			}
			if got := s.insert("probe zz-get al"); strings.Join(got, " ") != "probe zz-get alpha" {
				t.Errorf("probe zz-get al<TAB> gave %q", got)
			}
		})
	}
}

func TestShell_Zsh_UserAliases(t *testing.T) {
	s := zshSession(t, aliasEnv(t)...)
	if got := s.insert("probe zz-g"); strings.Join(got, " ") != "probe zz-get" {
		t.Errorf("probe zz-g<TAB> gave %q", got)
	}
	if got := s.insert("probe zz-get al"); strings.Join(got, " ") != "probe zz-get alpha" {
		t.Errorf("probe zz-get al<TAB> gave %q", got)
	}
}

func TestShell_Fish_UserAliases(t *testing.T) {
	fish := needShell(t, "fish", "fish")
	_, _, work := shellEnv(t)
	path := writeScript(t, "fish", "probe.fish")
	for line, want := range map[string][]string{
		"probe zz":      {"zz-get", "zz-out"},
		"probe zz-get ": {"alpha", "beta", "gamma ray"},
	} {
		cmd := exec.Command(fish, "--no-config", "-c", `source $CLIX_SCRIPT; and complete --do-complete=$CLIX_LINE`)
		cmd.Env = shellEnviron(t, append(aliasEnv(t), "CLIX_SCRIPT="+path, "CLIX_LINE="+line)...)
		cmd.Dir = work
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("fish: %v\n%s", err, out)
		}
		var got []string
		for _, l := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
			if v, _, _ := strings.Cut(l, "\t"); v != "" {
				got = append(got, v)
			}
		}
		assertSet(t, "fish "+line, got, want)
	}
}
