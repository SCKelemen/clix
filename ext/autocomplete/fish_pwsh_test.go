package autocomplete

import (
	"os/exec"
	"strings"
	"testing"
)

// candidateCase is a completion request and what the shell must and must
// not offer for it.
type candidateCase struct {
	name, line string
	want, not  []string
	exact      bool // want is the complete set
}

func checkCandidates(t *testing.T, c candidateCase, got []string) {
	t.Helper()
	t.Logf("%q: %q", c.line, got)
	if c.exact {
		assertSet(t, "candidates for "+c.line, got, c.want)
		return
	}
	for _, w := range c.want {
		found := false
		for _, g := range got {
			if g == w || strings.HasSuffix(w, "*") && strings.Contains(g, strings.TrimSuffix(w, "*")) {
				found = true
			}
		}
		if !found {
			t.Errorf("candidates for %q lack %q: %q", c.line, w, got)
		}
	}
	for _, w := range c.not {
		for _, g := range got {
			if g == w {
				t.Errorf("candidates for %q include %q: %q", c.line, w, got)
			}
		}
	}
}

// fishCases: "complete --do-complete" prints each candidate unescaped,
// as the whole token, with its description after a tab.
var fishCases = []candidateCase{
	{"TopLevel", "probe ", topLevel, nil, true},
	{"NestedCommands", "probe widgets ", []string{"get", "list", "ls"}, nil, true},
	{"Aliases", "probe w ", []string{"get", "list", "ls"}, nil, true},
	{"FlagsInScope", "probe widgets get -", allFlagsOfGet, nil, true},
	{"FlagsOutOfScope", "probe rm --", []string{"--force", "--path", "--help", "--verbose", "--org"}, nil, true},
	{"FlagValueHook", "probe widgets get --output ", []string{"json", "yaml"}, nil, true},
	{"FlagValueEquals", "probe widgets get --output=", []string{"--output=json", "--output=yaml"}, nil, true},
	{"PositionalHook", "probe widgets get ", []string{"alpha", "beta", "gamma ray"}, nil, true},
	{"WeirdNames", "probe weird ", weirdNames, nil, true},
	{"FileFallback", "probe rm fi", []string{"file-one.txt"}, nil, false},
	{"AfterDoubleDash", "probe rm -- -", []string{"-x.txt"}, []string{"--force"}, false},
	{"HiddenOmitted", "probe in", nil, []string{"internal-debug"}, false},
	{"NoFilesForCommands", "probe widgets ", nil, []string{"file-one.txt", "dir-a/"}, false},
}

// TestShell_Fish asks fish for completions with complete --do-complete
// after loading the script (CI only: fish is not installed everywhere).
func TestShell_Fish(t *testing.T) {
	fish := needShell(t, "fish", "fish")
	_, _, work := shellEnv(t)
	path := writeScript(t, "fish", "probe.fish")
	for _, c := range fishCases {
		t.Run(c.name, func(t *testing.T) {
			cmd := exec.Command(fish, "--no-config", "-c", `source $CLIX_SCRIPT; and complete --do-complete=$CLIX_LINE`)
			cmd.Env = append(shellEnviron(t), "CLIX_SCRIPT="+path, "CLIX_LINE="+c.line)
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
			checkCandidates(t, c, got)
		})
	}
	assertNotPwned(t)
}

// pwshCases: TabExpansion2 returns CompletionText, which PowerShell
// inserts as is, so values that need it are single-quoted.
var pwshCases = []candidateCase{
	{"TopLevel", "probe ", topLevel, nil, true},
	{"NestedCommands", "probe widgets ", []string{"get", "list", "ls"}, nil, true},
	{"Aliases", "probe w ", []string{"get", "list", "ls"}, nil, true},
	{"FlagsInScope", "probe widgets get -", allFlagsOfGet, nil, true},
	{"FlagsOutOfScope", "probe rm --", []string{"--force", "--path", "--help", "--verbose", "--org"}, nil, true},
	{"FlagValueHook", "probe widgets get --output ", []string{"json", "yaml"}, nil, true},
	{"FlagValueEquals", "probe widgets get --output=", []string{"--output=json", "--output=yaml"}, nil, true},
	{"PositionalHook", "probe widgets get ", []string{"alpha", "beta", "'gamma ray'"}, nil, true},
	{"WeirdNames", "probe weird ", []string{`'say"hi'`, "'it''s'", "'$(touch pwned)'", "'two words'", "x:y", `back\slash`}, nil, true},
	{"QuotedWord", "probe weird 'tw", []string{"'two words'"}, nil, true},
	{"FileFallback", "probe rm fi", []string{"file-one.txt*"}, nil, false},
	{"AfterDoubleDash", "probe rm -- -", nil, []string{"--force"}, false},
	{"HiddenOmitted", "probe in", nil, []string{"internal-debug"}, false},
	{"NoFilesForCommands", "probe nosuch ", nil, []string{"file-one.txt", "./file-one.txt"}, false},
}

// psDriver loads the script and prints the completion text of every
// match for $env:CLIX_LINE with the cursor at its end.
const psDriver = `. $env:CLIX_SCRIPT
$line = $env:CLIX_LINE
$result = TabExpansion2 -inputScript $line -cursorColumn $line.Length
foreach ($match in $result.CompletionMatches) { $match.CompletionText }`

// TestShell_PowerShell asks PowerShell for completions with TabExpansion2
// after loading the script (CI only: pwsh is not installed everywhere).
func TestShell_PowerShell(t *testing.T) {
	pwsh := needShell(t, "pwsh", "pwsh")
	_, _, work := shellEnv(t)
	path := writeScript(t, "powershell", "probe.ps1")
	for _, c := range pwshCases {
		t.Run(c.name, func(t *testing.T) {
			cmd := exec.Command(pwsh, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", psDriver)
			cmd.Env = append(shellEnviron(t), "CLIX_SCRIPT="+path, "CLIX_LINE="+c.line)
			cmd.Dir = work
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("pwsh: %v\n%s", err, out)
			}
			var got []string
			for _, l := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
				if l != "" {
					got = append(got, l)
				}
			}
			checkCandidates(t, c, got)
		})
	}
	assertNotPwned(t)
}
