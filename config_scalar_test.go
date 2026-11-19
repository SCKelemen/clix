package clix_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

func loadConfig(t *testing.T, body string) (*clix.ConfigManager, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	m := clix.NewConfigManager("x")
	return m, m.Load(path)
}

// YAML scalars were decoded into Go values and printed with %v, so a
// string flag got "1.1" for "1.10", "493" for "0755" and a time for a date.
func TestConfigScalarsKeepTheirText(t *testing.T) {
	for _, v := range []string{"1.10", "1.0", "0755", "1e3", "12345678901234567890", "2024-01-02", "0x1F", "True", "-0", ".inf"} {
		t.Run(v, func(t *testing.T) {
			app, _ := newTestApp(t, "myapp")
			writeConfigFile(t, app, "tag: "+v+"\n")
			var tag string
			cmd := clix.NewCommand("ls", clix.WithCommandRun(noop))
			cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "tag"}, Value: &tag})
			app.Root.AddCommand(cmd)
			if err := app.Run(bg, []string{"ls"}); err != nil {
				t.Fatal(err)
			}
			if tag != v {
				t.Fatalf("--tag = %q, want %q", tag, v)
			}
			if got, _ := app.Config.Get("tag"); got != v {
				t.Fatalf("Config.Get = %q, want %q", got, v)
			}
		})
	}
}

// Numeric flags still read YAML's numeric value, as before.
func TestConfigNumbersForNumericFlags(t *testing.T) {
	cases := []struct {
		yaml string
		want int
	}{
		{"0755", 493}, {"0x1F", 31}, {"0o17", 15}, {"1e3", 1000}, {"1_000", 1000}, {"8080", 8080}, {"'0755'", 755},
	}
	for _, c := range cases {
		t.Run(c.yaml, func(t *testing.T) {
			app, _ := newTestApp(t, "myapp")
			writeConfigFile(t, app, "n: "+c.yaml+"\nratio: 1.10\nraw: 0x10\n")
			var n int
			var ratio float64
			var viaCtx int
			cmd := clix.NewCommand("ls", clix.WithCommandRun(func(ctx *clix.Context) error {
				viaCtx, _, _ = ctx.EffectiveInt("raw")
				return nil
			}))
			cmd.Flags.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "n"}, Value: &n})
			cmd.Flags.Float64Var(clix.Float64VarOptions{FlagOptions: clix.FlagOptions{Name: "ratio"}, Value: &ratio})
			app.Root.AddCommand(cmd)
			if err := app.Run(bg, []string{"ls"}); err != nil {
				t.Fatal(err)
			}
			if n != c.want || ratio != 1.1 || viaCtx != 16 {
				t.Fatalf("n = %d (want %d), ratio = %v, EffectiveInt(raw) = %d", n, c.want, ratio, viaCtx)
			}
			if got, _ := app.Config.Int("raw"); got != 16 {
				t.Fatalf("Config.Int(raw) = %d", got)
			}
		})
	}
}

// Non-string keys were flattened with %v into "map[80:http]".
func TestConfigNonStringKeysAndListsOfMaps(t *testing.T) {
	m, err := loadConfig(t, "ports:\n  80: http\n  443: https\nhosts:\n  - name: a\n    port: 1\n  - name: b\ntags: [x, 1.10, ~]\n")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"ports.80": "http", "ports.443": "https",
		"hosts.0.name": "a", "hosts.0.port": "1", "hosts.1.name": "b",
		"tags": "x,1.10,",
	}
	got := m.Values()
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q (values %v)", k, got[k], v, got)
		}
	}
	if len(got) != len(want) {
		t.Errorf("values = %v", got)
	}
}

// The same key nested and dotted was resolved by map iteration order.
func TestConfigDuplicateFlattenedKeyIsAnError(t *testing.T) {
	for _, body := range []string{
		"project:\n  name: nested\nproject.name: dotted\n",
		"project.name: dotted\nproject:\n  name: nested\n",
		"a: 1\na: 2\n",
	} {
		_, err := loadConfig(t, body)
		if err == nil || !strings.Contains(err.Error(), "defined more than once") {
			t.Errorf("Load(%q) = %v, want a duplicate-key error", body, err)
		}
	}
}

// Keys brought in by a YAML merge key may be overridden; aliases resolve.
func TestConfigMergeKeysAndAliases(t *testing.T) {
	m, err := loadConfig(t, "base: &b\n  region: eu\n  size: 2\nprod:\n  <<: *b\n  size: 5\ncopy: *b\n")
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{"prod.region": "eu", "prod.size": "5", "copy.size": "2", "base.size": "2"} {
		if got, _ := m.Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

// Parse errors did not name the file.
func TestConfigErrorsNameTheFile(t *testing.T) {
	for _, body := range []string{"a: [unclosed\n", "- just\n- a list\n", "a: 1\na: 2\n"} {
		app, _ := newTestApp(t, "myapp")
		path := writeConfigFile(t, app, body)
		app.Root.AddCommand(clix.NewCommand("ls", clix.WithCommandRun(noop)))
		if err := app.Run(bg, []string{"ls"}); err == nil || !strings.Contains(err.Error(), path) {
			t.Errorf("Run with %q = %v, want an error naming %s", body, err, path)
		}
	}
}

// Merge keys merged deeply: "child: {<<: *b, y: {w: 3}}" still got
// child.y.z from the base, and an explicit "x: ~" did not override the
// merged x.
func TestConfigMergeKeysAreShallow(t *testing.T) {
	cases := []struct {
		body string
		want map[string]string
	}{
		{"base: &b {x: 1, y: {z: 2}}\nchild:\n  <<: *b\n  y: {w: 3}\n", map[string]string{"base.x": "1", "base.y.z": "2", "child.x": "1", "child.y.w": "3"}},
		{"base: &b {x: 1}\nchild:\n  <<: *b\n  x: ~\n", map[string]string{"base.x": "1"}},
		{"a: &a {k: from-a, only-a: 1}\nb: &b {k: from-b, only-b: 2}\nc:\n  <<: [*a, *b]\n", map[string]string{"a.k": "from-a", "a.only-a": "1", "b.k": "from-b", "b.only-b": "2", "c.k": "from-a", "c.only-a": "1", "c.only-b": "2"}},
		{"base: &b {x: 1}\nmid: &m {<<: *b, y: 2}\ntop: {<<: *m, z: 3}\n", map[string]string{"base.x": "1", "mid.x": "1", "mid.y": "2", "top.x": "1", "top.y": "2", "top.z": "3"}},
	}
	for _, c := range cases {
		m, err := loadConfig(t, c.body)
		if err != nil {
			t.Fatalf("%q: %v", c.body, err)
		}
		if got := m.Values(); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q:\n got %v\nwant %v", c.body, got, c.want)
		}
	}
}
