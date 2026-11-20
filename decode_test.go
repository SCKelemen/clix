package clix_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
)

// strictLevel implements encoding.TextUnmarshaler.
type strictLevel string

func (l *strictLevel) UnmarshalText(b []byte) error {
	if len(b) == 0 {
		return errors.New("empty level")
	}
	*l = strictLevel("level:" + string(b))
	return nil
}

type strictPort struct {
	Name string `json:"name"`
	Port int    `json:"port"`
}

type strictMeta struct {
	Owner string `json:"owner"`
}

type strictSpec struct {
	strictMeta                   // embedded: its fields are inlined
	Name       string            `json:"name"`
	Replicas   int               `json:"replicas"`
	Version    string            `json:"version"`
	Mode       int               `json:"mode"`
	Ratio      float64           `json:"ratio"`
	Enabled    bool              `json:"enabled"`
	Timeout    time.Duration     `json:"timeout"`
	Labels     map[string]string `json:"labels"`
	Ports      []strictPort      `json:"ports"`
	Level      strictLevel       `json:"level"`
	Any        any               `json:"any"`
	Ptr        *strictPort       `json:"ptr"`
	NoTag      string
}

func TestUnmarshalStrict(t *testing.T) {
	tests := []struct {
		name  string
		doc   string
		check func(s strictSpec) bool
	}{
		{"YAML", "name: web\nreplicas: 3\nenabled: true\nports:\n- name: http\n  port: 80\n", func(s strictSpec) bool {
			return s.Name == "web" && s.Replicas == 3 && s.Enabled && reflect.DeepEqual(s.Ports, []strictPort{{"http", 80}})
		}},
		{"JSON", `{"name": "web", "replicas": 3, "ports": [{"name": "http", "port": 80}], "labels": {"a": "b"}}`, func(s strictSpec) bool {
			return s.Name == "web" && s.Replicas == 3 && s.Labels["a"] == "b" && len(s.Ports) == 1
		}},
		{"text keeps what was written", "version: 1.10\nname: 0755\nlabels: {a: 1.10, b: True, c: 0x1F}\n", func(s strictSpec) bool {
			return s.Version == "1.10" && s.Name == "0755" && s.Labels["a"] == "1.10" && s.Labels["b"] == "True" && s.Labels["c"] == "0x1F"
		}},
		{"numbers read YAML's number", "mode: 0755\nreplicas: 0x1F\nratio: 1.10\n", func(s strictSpec) bool {
			return s.Mode == 493 && s.Replicas == 31 && s.Ratio == 1.1
		}},
		{"a TextUnmarshaler gets the text", "level: 2.0\n", func(s strictSpec) bool { return s.Level == "level:2.0" }},
		{"a TextUnmarshaler gets a string", "level: debug\n", func(s strictSpec) bool { return s.Level == "level:debug" }},
		{"durations", "timeout: 1m30s\n", func(s strictSpec) bool { return s.Timeout == 90*time.Second }},
		{"embedded fields", "owner: ops\n", func(s strictSpec) bool { return s.Owner == "ops" }},
		{"an untagged field by its Go name", "NoTag: x\n", func(s strictSpec) bool { return s.NoTag == "x" }},
		{"any keeps YAML's types", "any: {n: 1, b: true, s: x}\n", func(s strictSpec) bool {
			m, ok := s.Any.(map[string]any)
			return ok && m["n"] == 1.0 && m["b"] == true && m["s"] == "x"
		}},
		{"null", "name: ~\nptr: null\n", func(s strictSpec) bool { return s.Name == "" && s.Ptr == nil }},
		{"merge keys", "ptr:\n  <<: {name: http, port: 80}\n  port: 8080\n", func(s strictSpec) bool {
			return s.Ptr != nil && s.Ptr.Name == "http" && s.Ptr.Port == 8080
		}},
		{"aliases", "ports: [&p {name: a, port: 1}, *p]\n", func(s strictSpec) bool { return len(s.Ports) == 2 && s.Ports[1].Name == "a" }},
		{"an empty document changes nothing", "", func(s strictSpec) bool { return s.Name == "default" && s.Replicas == 1 }},
		{"only comments", "# nothing\n", func(s strictSpec) bool { return s.Name == "default" }},
		{"defaults survive", "replicas: 5\n", func(s strictSpec) bool { return s.Name == "default" && s.Replicas == 5 }},
		{"Windows paths", "name: C:\\data\\web\nversion: 'D:\\x'\n", func(s strictSpec) bool { return s.Name == `C:\data\web` && s.Version == `D:\x` }},
	}
	for _, tt := range tests {
		if tt.check == nil {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			s := strictSpec{Name: "default", Replicas: 1}
			if err := clix.UnmarshalStrict([]byte(tt.doc), &s); err != nil {
				t.Fatal(err)
			}
			if !tt.check(s) {
				t.Errorf("got %+v", s)
			}
		})
	}
}

// Every unknown field, duplicate key and value of the wrong type is
// reported, with its path, and the target is left alone.
func TestUnmarshalStrictErrors(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want []string // FieldError messages, in order
	}{
		{"an unknown field", "name: web\nreplcas: 3\n", []string{`unknown field "replcas"`}},
		{"every unknown field, nested and in lists", "nmae: x\nports:\n- name: a\n  prot: 1\n- naem: b\nptr: {port: 1, extra: 2}\n", []string{
			`unknown field "nmae"`, `unknown field "ports[0].prot"`, `unknown field "ports[1].naem"`, `unknown field "ptr.extra"`}},
		{"names are case-sensitive", "Name: x\nnotag: y\n", []string{`unknown field "Name"`, `unknown field "notag"`}},
		{"a duplicate key", "name: a\nreplicas: 1\nname: b\n", []string{`duplicate field "name"`}},
		{"a nested duplicate key", "ptr:\n  port: 1\n  port: 2\n", []string{`duplicate field "ptr.port"`}},
		{"a duplicate key in JSON", `{"name": "a", "name": "b"}`, []string{`duplicate field "name"`}},
		{"a duplicate key the target ignores", "labels: {a: 1, a: 2}\n", []string{`duplicate field "labels.a"`}},
		{"1 and '1' are the same key", "labels: {1: a, '1': b}\n", []string{`duplicate field "labels.1"`}},
		{"a string for an int", "replicas: '3'\n", []string{"replicas: expected int, got a string"}},
		{"a list for an int", "replicas: [1]\n", []string{"replicas: expected int, got a list"}},
		{"a mapping for a list", "ports: {a: 1}\n", []string{"ports: expected []clix_test.strictPort, got a mapping"}},
		{"a string for a bool", "enabled: yes\n", []string{"enabled: expected bool, got a string"}},
		{"a number for a duration", "timeout: 30\n", []string{"timeout: expected a duration such as 30s or 1h30m"}},
		{"bad duration text", "timeout: soon\n", []string{"timeout: expected a duration such as 30s or 1h30m"}},
		{"a TextUnmarshaler's error", "level: ''\n", []string{"level: empty level"}},
		{"an overflow", "replicas: 99999999999999999999\n", []string{"replicas: "}},
		{".inf for a float", "ratio: .inf\n", []string{"ratio: expected float64, got a string"}},
		{"a list for the document", "- a\n", []string{"expected clix_test.strictSpec, got a list"}},
		{"all kinds at once", "name: a\nname: b\nbogus: 1\nreplicas: x\n", []string{`unknown field "bogus"`, `duplicate field "name"`, "replicas: expected int, got a string"}},
		{"unknown keys pulled in by a merge key", "ptr:\n  <<: {name: a, typo: 1}\n", []string{`unknown field "ptr.typo"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := strictSpec{Name: "default", Replicas: 1}
			err := clix.UnmarshalStrict([]byte(tt.doc), &s)
			var strict *clix.StrictError
			if !errors.As(err, &strict) {
				t.Fatalf("err = %v, want a *StrictError", err)
			}
			if len(strict.Fields) != len(tt.want) {
				t.Fatalf("got %d problems (%v), want %d", len(strict.Fields), err, len(tt.want))
			}
			for i, w := range tt.want {
				if got := strict.Fields[i].Error(); !strings.HasPrefix(got, w) {
					t.Errorf("problem %d = %q, want %q", i, got, w)
				}
			}
			if s.Name != "default" || s.Replicas != 1 || s.Ports != nil {
				t.Errorf("the target changed: %+v", s)
			}
			if !strings.HasPrefix(err.Error(), "strict decoding error: ") {
				t.Errorf("message %q", err)
			}
		})
	}
}

// errors.Is finds the kinds of problems in a StrictError.
func TestStrictErrorIs(t *testing.T) {
	var s strictSpec
	err := clix.UnmarshalStrict([]byte("bogus: 1\nname: a\nname: b\n"), &s)
	if !errors.Is(err, clix.ErrUnknownField) || !errors.Is(err, clix.ErrDuplicateField) {
		t.Errorf("errors.Is fails for %v", err)
	}
	err = clix.UnmarshalStrict([]byte("bogus: 1\n"), &s)
	if errors.Is(err, clix.ErrDuplicateField) {
		t.Errorf("%v is not a duplicate", err)
	}
}

// Documents that are not one valid YAML or JSON document, and targets
// that are not pointers, are errors but not StrictErrors.
func TestUnmarshalStrictInvalid(t *testing.T) {
	var s strictSpec
	for _, doc := range []string{"name: [a\n", "{\"name\": \"a\"\n", "name: a\n---\nname: b\n", "a: b: c\n"} {
		if err := clix.UnmarshalStrict([]byte(doc), &s); err == nil {
			t.Errorf("%q: no error", doc)
		}
	}
	for _, v := range []any{nil, s, (*strictSpec)(nil)} {
		if err := clix.UnmarshalStrict([]byte("name: a\n"), v); err == nil || !strings.Contains(err.Error(), "non-nil pointer") {
			t.Errorf("%T: err = %v", v, err)
		}
	}
}

// YAML aliases cannot expand a small document without end, through
// values or merge keys.
func TestUnmarshalStrictAliasBombs(t *testing.T) {
	values := "a: &a [x, x, x, x, x, x, x, x, x, x]\n"
	merges := "a: &a {k0: 1, k1: 1}\n"
	prev := "a"
	for i := range 9 {
		name := fmt.Sprintf("b%d", i)
		values += fmt.Sprintf("%s: &%s [*%s, *%s, *%s, *%s, *%s, *%s, *%s, *%s, *%s, *%s]\n", name, name, prev, prev, prev, prev, prev, prev, prev, prev, prev, prev)
		merges += fmt.Sprintf("%s: &%s {<<: [*%s, *%s, *%s, *%s, *%s, *%s, *%s, *%s, *%s, *%s]}\n", name, name, prev, prev, prev, prev, prev, prev, prev, prev, prev, prev)
		prev = name
	}
	for name, doc := range map[string]string{"values": values, "merge keys": merges} {
		t.Run(name, func(t *testing.T) {
			var v map[string]any
			err := clix.UnmarshalStrict([]byte(doc), &v)
			if err == nil || !strings.Contains(err.Error(), "expands to more than") {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

// A StrictError lists at most 64 problems.
func TestUnmarshalStrictTooManyProblems(t *testing.T) {
	var b strings.Builder
	for i := range 100 {
		fmt.Fprintf(&b, "bogus%d: 1\n", i)
	}
	var s strictSpec
	err := clix.UnmarshalStrict([]byte(b.String()), &s)
	var strict *clix.StrictError
	if !errors.As(err, &strict) || len(strict.Fields) != 65 || !strings.HasSuffix(err.Error(), "too many errors") {
		t.Fatalf("err = %v", err)
	}
}

type configServer struct {
	Host    string        `json:"host"`
	Port    int           `json:"port"`
	Mode    int           `json:"mode"`
	ModeStr string        `json:"mode_text"`
	Version string        `json:"version"`
	Timeout time.Duration `json:"timeout"`
	Tags    []string      `json:"tags"`
	TLS     struct {
		CertFile string `json:"cert_file"`
	} `json:"tls"`
	Hosts []configHost `json:"hosts"`
}

type configHost struct {
	Name string `json:"name"`
	Port int    `json:"port"`
}

// ConfigManager.Decode decodes a section of the config file strictly,
// keeping the text written in the file.
func TestConfigDecode(t *testing.T) {
	path := writeTestFile(t, t.TempDir(), "config.yaml", `
server:
  host: example.com
  mode: 0755
  mode_text: 0755
  version: 1.10
  timeout: 90s
  tags: [a, "b,c"]
  tls:
    cert_file: /etc/cert.pem
  hosts:
    - name: one
      port: 1
    - name: two
other: x
flat.key: 1
`)
	m := clix.NewConfigManager("myapp")
	if err := m.Load(path); err != nil {
		t.Fatal(err)
	}

	s := configServer{Port: 8080}
	if err := m.Decode("server", &s); err != nil {
		t.Fatal(err)
	}
	want := configServer{Host: "example.com", Port: 8080, Mode: 493, ModeStr: "0755", Version: "1.10", Timeout: 90 * time.Second,
		Tags: []string{"a", "b,c"}, Hosts: []configHost{{"one", 1}, {"two", 0}}}
	want.TLS.CertFile = "/etc/cert.pem"
	if !reflect.DeepEqual(s, want) {
		t.Errorf("got  %+v\nwant %+v", s, want)
	}

	t.Run("a value", func(t *testing.T) {
		var host string
		if err := m.Decode("server.host", &host); err != nil || host != "example.com" {
			t.Errorf("host = %q, err = %v", host, err)
		}
	})
	t.Run("a list of maps", func(t *testing.T) {
		var hosts []configHost
		if err := m.Decode("server.hosts", &hosts); err != nil || len(hosts) != 2 || hosts[1].Name != "two" {
			t.Errorf("hosts = %+v, err = %v", hosts, err)
		}
	})
	t.Run("the whole file", func(t *testing.T) {
		var all map[string]any
		if err := m.Decode("", &all); err != nil {
			t.Fatal(err)
		}
		if all["other"] != "x" || all["flat"].(map[string]any)["key"] != 1.0 {
			t.Errorf("all = %v", all)
		}
	})
	t.Run("nothing under the prefix", func(t *testing.T) {
		s := configServer{Port: 1}
		if err := m.Decode("missing", &s); err != nil || s.Port != 1 {
			t.Errorf("s = %+v, err = %v", s, err)
		}
	})
	t.Run("Set values", func(t *testing.T) {
		m := clix.NewConfigManager("myapp")
		m.Set("server.port", "9090")
		m.Set("server.version", "1.10")
		m.Set("server.mode", "0755")
		var s configServer
		err := m.Decode("server", &s)
		// "0755" from Set is quoted when saved, so it is text, not a number.
		var strict *clix.StrictError
		if !errors.As(err, &strict) || len(strict.Fields) != 1 || strict.Fields[0].Path != "server.mode" {
			t.Fatalf("err = %v", err)
		}
		m.Delete("server.mode")
		if err := m.Decode("server", &s); err != nil || s.Port != 9090 || s.Version != "1.10" {
			t.Errorf("s = %+v, err = %v", s, err)
		}
	})
}

// Problems in the config file name the file and the full config key.
func TestConfigDecodeErrors(t *testing.T) {
	path := writeTestFile(t, t.TempDir(), "config.yaml", `
server:
  hots: example.com
  port: "80"
  tls:
    certfile: x
  hosts:
    - name: one
      prot: 1
`)
	m := clix.NewConfigManager("myapp")
	if err := m.Load(path); err != nil {
		t.Fatal(err)
	}
	s := configServer{Host: "default"}
	err := m.Decode("server", &s)
	var strict *clix.StrictError
	if !errors.As(err, &strict) {
		t.Fatalf("err = %v", err)
	}
	want := []string{
		`unknown field "server.hosts[0].prot"`,
		`unknown field "server.hots"`,
		"server.port: expected int, got a string",
		`unknown field "server.tls.certfile"`,
	}
	var got []string
	for _, f := range strict.Fields {
		got = append(got, f.Error())
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	if !strings.HasPrefix(err.Error(), "config file "+filepath.Clean(path)+": strict decoding error: ") {
		t.Errorf("message %q", err)
	}
	if s.Host != "default" {
		t.Errorf("the target changed: %+v", s)
	}
}
