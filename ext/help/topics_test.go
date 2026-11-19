package help

import (
	"context"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2/ext/autocomplete"
)

// "help <topic>" shows a topic, root help lists them, and shell completion
// offers them after the commands, through the __complete protocol.
func TestHelpTopicsE2E(t *testing.T) {
	app, out, _ := e2eHelpApp(t)
	app.AddExtension(autocomplete.Extension{})
	app.AddHelpTopic("formatting", "Formatting output", "Use --format json, yaml or text.")

	root := helpRun(t, app, out, "help")
	if !strings.Contains(root, "ADDITIONAL HELP TOPICS\n  environment          Environment variables that change how the program behaves\n  exit-codes           Exit codes and what they mean\n  formatting           Formatting output\n") {
		t.Fatalf("root help:\n%s", root)
	}
	if got := helpRun(t, app, out, "help", "formatting"); got != "Formatting output\n\nUse --format json, yaml or text.\n" {
		t.Fatalf("help formatting = %q", got)
	}
	if got := helpRun(t, app, out, "help", "--command", "exit-codes"); !strings.Contains(got, "  2    A usage error") {
		t.Fatalf("help --command exit-codes:\n%s", got)
	}
	if got := helpRun(t, app, out, "help", "environment"); !strings.Contains(got, "PROBE_FORMAT") || !strings.Contains(got, "PROBE_VERBOSE") || !strings.Contains(got, "PROBE_DISABLE_PROMPTS") {
		t.Fatalf("help environment:\n%s", got)
	}
	// A hidden command still beats a topic of its name.
	app.AddHelpTopic("secret", "A topic", "never shown")
	if got := helpRun(t, app, out, "help", "secret"); !strings.HasPrefix(got, "Hidden tools") {
		t.Fatalf("help secret:\n%s", got)
	}

	complete := func(cur string, words ...string) []string {
		t.Helper()
		out.Reset()
		if err := app.Run(context.Background(), append([]string{"__complete", "--cur=" + cur, "--", "probe", "help"}, words...)); err != nil {
			t.Fatal(err)
		}
		return strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	}
	got := complete("")
	if strings.Join(got, "|") != "autocomplete\tGenerate shell completion script|help\tShow help for commands|widgets\tManage widgets|environment\tEnvironment variables that change how the program behaves|exit-codes\tExit codes and what they mean|formatting\tFormatting output|:nofiles" {
		t.Errorf("help <TAB>: %q", got)
	}
	if got := complete("e"); strings.Join(got, "|") != "environment\tEnvironment variables that change how the program behaves|exit-codes\tExit codes and what they mean|:nofiles" {
		t.Errorf("help e<TAB>: %q", got)
	}
	if got := complete("", "widgets"); strings.Contains(strings.Join(got, "|"), "environment") {
		t.Errorf("help widgets <TAB> offers a topic: %q", got)
	}
}
