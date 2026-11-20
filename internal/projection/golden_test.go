package projection

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/SCKelemen/clix/v2/internal/gcloudtest"
	"github.com/SCKelemen/clix/v2/internal/resource"
)

// update rewrites the golden files in testdata/golden (go test -update),
// the flag that also records gcloud's results (see gcloudtest.Golden).
var update = gcloudtest.Update

// goldenCases print testdata/instances.json. Each output is in
// testdata/golden/<name>.txt; TestGoldenMatchesGcloud checks that each is
// what gcloud prints.
var goldenCases = []struct{ name, format string }{
	{"table", "table(name, zone.basename():label=ZONE, machineType.basename():label=MACHINE_TYPE, status, networkInterfaces[0].networkIP:label=INTERNAL_IP, labels.env)"},
	{"table_default_labels", "table(name, labels.env, labels.team, networkInterfaces.name, networkInterfaces[0].name)"},
	{"table_box", "table[box,title=Instances](name, status:align=center, cpus:align=right, memoryGb)"},
	{"table_all_box", "table[all-box](name, description)"},
	{"table_sorted", "table(status:sort=1, name:sort=2:reverse, cpus)"},
	{"table_no_heading", "table[no-heading,pad=4](name, tags.items.join(','), tags.items.len())"},
	{"table_multiline", "table(name, description, owner)"},
	{"table_values", "table(cpus, memoryGb, preemptible, labels, tags.items, owner)"},
	{"value", "value(name, tags.items, labels, memoryGb)"},
	{"value_quote", "value[quote,separator=','](name, description)"},
	{"value_one", "value(networkInterfaces[0].accessConfigs[0].natIP)"},
	{"csv", "csv(name, status, cpus, memoryGb, preemptible, tags.items, labels, description)"},
	{"csv_options", "csv[no-heading,separator=';',delimiter='|'](name, tags.items)"},
	{"csv_labels", "csv(name:label=Instance, zone.basename():label=Zone)"},
	{"flattened", "flattened"},
	{"flattened_projection", "flattened(name, networkInterfaces[].networkIP, labels)"},
	{"flattened_options", "flattened[no-pad,separator=' = '](name, status, description)"},
	{"list", "list"},
	{"list_projection", "list[title=Instances](name, status)"},
	{"list_compact", "list[compact](name, zone.basename())"},
}

func goldenData(t testing.TB) any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "instances.json"))
	if err != nil {
		t.Fatal(err)
	}
	v, err := resource.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestGolden(t *testing.T) {
	data := goldenData(t)
	for _, c := range goldenCases {
		t.Run(c.name, func(t *testing.T) {
			f, err := Parse(c.format)
			if err != nil {
				t.Fatalf("Parse(%q): %v", c.format, err)
			}
			var b bytes.Buffer
			if err := f.Print(&b, data, Options{UTF8: true}); err != nil {
				t.Fatalf("Print(%q): %v", c.format, err)
			}
			path := filepath.Join("testdata", "golden", c.name+".txt")
			if *update {
				if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if b.String() != string(want) {
				t.Errorf("%s:\n--- got\n%s--- want (%s)\n%s---", c.format, b.String(), path, want)
			}
		})
	}
}

// TestGoldenMatchesGcloud checks that the golden files are what gcloud
// prints for the same formats and data.
func TestGoldenMatchesGcloud(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "instances.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []printCase
	for _, c := range goldenCases {
		cases = append(cases, printCase{c.format, string(raw)})
	}
	var got []printResult
	gcloudtest.Run(t, printScript, cases, &got)
	for i, c := range goldenCases {
		want, err := os.ReadFile(filepath.Join("testdata", "golden", c.name+".txt"))
		if err != nil {
			t.Fatal(err)
		}
		if got[i].Error != "" || got[i].Out != string(want) {
			t.Errorf("%s: gcloud printed:\n%s%s\ngolden file:\n%s", c.format, got[i].Out, got[i].Error, want)
		}
	}
}
