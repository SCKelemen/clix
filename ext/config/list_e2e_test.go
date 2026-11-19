package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"
)

// config list used to drop "a" when "a.b" was also set.
func TestConfigE2E_ListKeepsKeyThatIsAlsoAPrefix(t *testing.T) {
	app, out, _ := e2eApp(t, "")
	run(t, app, out, "config", "set", "a", "1")
	run(t, app, out, "config", "set", "a.b", "2")
	run(t, app, out, "config", "set", "x.y", "3")
	run(t, app, out, "config", "set", "x.y.z", "4")
	run(t, app, out, "config", "set", "x.w", "5")

	want := "a: \"1\"\na.b: \"2\"\nx:\n  w: \"5\"\n  \"y\": \"3\"\n  y.z: \"4\"\n"
	if got := run(t, app, out, "config", "list"); got != want {
		t.Errorf("config list:\n%s\nwant:\n%s", got, want)
	}

	var j map[string]any
	if err := json.Unmarshal([]byte(run(t, app, out, "config", "list", "--format", "json")), &j); err != nil {
		t.Fatal(err)
	}
	if j["a"] != "1" || j["a.b"] != "2" || j["x"].(map[string]any)["y.z"] != "4" {
		t.Errorf("config list --format json: %v", j)
	}
}

// The hand-written YAML only quoted ':' '#' and edge spaces, so values such
// as "yes", "0123", "- x", "[a]", "null", "@x" and multi-line text changed
// type or broke the document.
func TestConfigE2E_ListYAMLRoundTrips(t *testing.T) {
	vals := map[string]string{
		"k1": "yes", "k2": "0123", "k3": "- x", "k4": "[a]", "k5": "null", "k6": "@x",
		"k7": "line1\nline2", "k8": "a: b", "k9": "# not a comment", "k10": " padded ", "k11": "30",
		"k12": "{x}", "k13": "*alias", "k14": "~", "k15": "'q'", "k16": `"dq"`, "k17": "1e3", "k18": "true",
		"yes": "key that looks like a bool", "nested.null": "under a plain key",
	}
	for _, format := range []string{"", "yaml", "text", "YAML"} {
		app, out, _ := e2eApp(t, "")
		for k, v := range vals {
			app.Config.Set(k, v)
		}
		args := []string{"config", "list"}
		if format != "" {
			args = append(args, "--format", format)
		}
		got := run(t, app, out, args...)
		var back map[string]any
		if err := yaml.Unmarshal([]byte(got), &back); err != nil {
			t.Fatalf("%q: output is not YAML: %v\n%s", args, err, got)
		}
		for k, v := range vals {
			node := any(back)
			if k == "nested.null" {
				node = back["nested"].(map[string]any)["null"]
			} else {
				node = back[k]
			}
			if s, ok := node.(string); !ok || s != v {
				t.Errorf("%q: %s: wrote %q, read back %#v", args, k, v, node)
			}
		}
	}
}

// A list is stable: the same values always print the same, sorted by key.
func TestConfigE2E_ListOrderIsStable(t *testing.T) {
	app, out, path := e2eApp(t, "")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("zeta: 1\nalpha:\n  b: 2\n  a: 1\nmid: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := "alpha:\n  a: \"1\"\n  b: \"2\"\nmid: x\nzeta: \"1\"\n"
	for i := 0; i < 5; i++ {
		if got := run(t, app, out, "config", "list"); got != want {
			t.Fatalf("run %d: config list:\n%s\nwant:\n%s", i, got, want)
		}
	}
	wantJSON := "{\n  \"alpha\": {\n    \"a\": \"1\",\n    \"b\": \"2\"\n  },\n  \"mid\": \"x\",\n  \"zeta\": \"1\"\n}\n"
	if got := run(t, app, out, "config", "list", "--format", "json"); got != wantJSON {
		t.Errorf("config list --format json:\n%s\nwant:\n%s", got, wantJSON)
	}
}

// --format JSON was ignored because the comparison was case-sensitive.
func TestConfigE2E_ListFormatIgnoresCase(t *testing.T) {
	for _, f := range []string{"JSON", "Json", "json"} {
		app, out, _ := e2eApp(t, "")
		app.Config.Set("url", "https://x.test/?a=1&b=2")
		got := run(t, app, out, "config", "list", "--format", f)
		if got != "{\n  \"url\": \"https://x.test/?a=1&b=2\"\n}\n" {
			t.Errorf("--format %s printed:\n%s", f, got)
		}
	}
}

func TestConfigE2E_ListEmpty(t *testing.T) {
	app, out, _ := e2eApp(t, "")
	if got := run(t, app, out, "config", "list"); got != "" {
		t.Errorf("config list with nothing stored printed %q", got)
	}
	if got := run(t, app, out, "config", "list", "-f", "json"); got != "{}\n" {
		t.Errorf("config list -f json with nothing stored printed %q", got)
	}
}
