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
// Without CLIX_GCLOUD_SDK the tests are skipped.
package gcloudtest

import (
	"bytes"
	"encoding/json"
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
