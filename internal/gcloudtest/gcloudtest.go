// Package gcloudtest runs Python code against an installed Google Cloud
// SDK, so that tests can compare clix's ports of gcloud's resource
// library (internal/resource, internal/filter and the format projections)
// with gcloud itself.
//
// The tests that use it are opt-in: set CLIX_GCLOUD_SDK to the SDK's root
// directory (the one with lib/googlecloudsdk, for example
// /opt/homebrew/share/google-cloud-sdk or "$(gcloud info
// --format='value(installation.sdk_root)')"), and CLIX_GCLOUD_PYTHON to
// the Python that gcloud runs with ("gcloud info
// --format='value(basic.python_location)'"); it defaults to python3.
// Without CLIX_GCLOUD_SDK, tests that use Run are skipped and tests that
// use Golden compare with gcloud's results recorded in testdata; record
// them again with "go test -run ... -update" and CLIX_GCLOUD_SDK set.
package gcloudtest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// prelude puts the SDK's libraries on the path. Python runs with -I, so
// nothing from the environment or the current directory is imported.
const prelude = `import sys, json
sys.path.insert(0, %q)
sys.path.insert(1, %q)
`

// Run runs script, a Python program that reads JSON from stdin and
// writes JSON to stdout, with the SDK's libraries importable, and decodes
// its output into out. It skips the test unless CLIX_GCLOUD_SDK is set.
func Run(t testing.TB, script string, in, out any) {
	t.Helper()
	sdk := os.Getenv("CLIX_GCLOUD_SDK")
	if sdk == "" {
		t.Skip("set CLIX_GCLOUD_SDK (and CLIX_GCLOUD_PYTHON) to compare with gcloud")
	}
	python := os.Getenv("CLIX_GCLOUD_PYTHON")
	if python == "" {
		python = "python3"
	}
	lib := filepath.Join(sdk, "lib")
	dir := t.TempDir()
	path := filepath.Join(dir, "gcloudtest.py")
	head := []byte(fmt.Sprintf(prelude, lib, filepath.Join(lib, "third_party")))
	if err := os.WriteFile(path, append(head, script...), 0o600); err != nil {
		t.Fatal(err)
	}
	input, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, "-I", path)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s: %v\n%s", python, err, stderr.String())
	}
	if err := json.Unmarshal(stdout.Bytes(), out); err != nil {
		t.Fatalf("decoding the output of %s: %v\n%s", python, err, stdout.String())
	}
}

// Update is the -update flag of the tests that use Golden: it records
// gcloud's output in their golden files.
var Update = flag.Bool("update", false, "record gcloud's results in testdata (needs CLIX_GCLOUD_SDK)")

// golden is the content of a golden file: a digest of the input and
// gcloud's output.
type golden struct {
	Input  string          `json:"input_sha256"`
	Output json.RawMessage `json:"output"`
}

// digest is the SHA-256 of the JSON encoding of in.
func digest(t testing.TB, in any) string {
	t.Helper()
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Golden is Run for tests that also run without gcloud: with
// CLIX_GCLOUD_SDK set it runs script with gcloud (and with -update records
// gcloud's output in the file at path); otherwise it reads the output
// recorded there, which must be for the same input.
func Golden(t testing.TB, path, script string, in, out any) {
	t.Helper()
	if os.Getenv("CLIX_GCLOUD_SDK") != "" || *Update {
		var raw json.RawMessage
		Run(t, script, in, &raw)
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatal(err)
		}
		if *Update {
			data, err := encodeGolden(golden{Input: digest(t, in), Output: raw})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (record it with -update and CLIX_GCLOUD_SDK set)", err)
	}
	var g golden
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	if g.Input != digest(t, in) {
		t.Fatalf("%s was recorded for other input: record it again with -update and CLIX_GCLOUD_SDK set", path)
	}
	if err := json.Unmarshal(g.Output, out); err != nil {
		t.Fatal(err)
	}
}

// encodeGolden writes g with the elements of an output list one per line,
// so that a change shows as a change of the lines it affects.
func encodeGolden(g golden) ([]byte, error) {
	var list []json.RawMessage
	if err := json.Unmarshal(g.Output, &list); err != nil {
		return json.MarshalIndent(g, "", " ")
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "{\"input_sha256\": %q, \"output\": [\n", g.Input)
	for i, e := range list {
		var c bytes.Buffer
		if err := json.Compact(&c, e); err != nil {
			return nil, err
		}
		b.Write(c.Bytes())
		if i < len(list)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString("]}\n")
	return b.Bytes(), nil
}
