package format_test

import (
	"bytes"
	"os/exec"
	"testing"
)

// --jq prints what "jq -r" prints for the --json output. Skipped when jq
// is not installed; internal/jq has the full differential tests.
func TestJQMatchesRealJQ(t *testing.T) {
	jqPath, err := exec.LookPath("jq")
	if err != nil {
		t.Skip("jq is not installed")
	}
	app, out, ran := writeApp(t)
	for _, tc := range []struct {
		fields string
		expr   string
	}{
		{"name,stars,owner,labels,createdAt,note,url,displayName", "."},
		{"name,owner.login", ".[] | {name, login: .owner.login}"},
		{"labels", "[.[].labels[] | .name] | join(\", \")"},
		{"name,stars", "map(select(.stars >= 7)) | length"},
		{"displayName", ".[] | .displayName | test(\"<\")"},
		{"createdAt", ".[0].createdAt"},
		{"name", "to_entries"},
	} {
		doc, err := runWrite(t, app, out, ran, "list", "--json", tc.fields)
		if err != nil {
			t.Fatal(err)
		}
		got, err := runWrite(t, app, out, ran, "list", "--json", tc.fields, "--jq", tc.expr)
		if err != nil {
			t.Fatalf("--jq %q: %v", tc.expr, err)
		}
		cmd := exec.Command(jqPath, "-r", tc.expr)
		cmd.Stdin = bytes.NewReader([]byte(doc))
		want, err := cmd.Output()
		if err != nil {
			t.Fatalf("jq -r %q: %v", tc.expr, err)
		}
		if got != string(want) {
			t.Errorf("--json %s --jq %q:\ngot:\n%s\njq -r:\n%s", tc.fields, tc.expr, got, want)
		}
	}
}
