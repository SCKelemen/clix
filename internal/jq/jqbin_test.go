package jq

import (
	"bytes"
	"errors"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// The differential tests compare Query.Write with the real jq (jq -r) on
// the same input. They are skipped when jq is not installed, and need jq
// 1.7 or later. When jq was built without decNumber (so it does not keep
// number literals as written), cases marked literal are skipped.

type jqBinary struct {
	path    string
	version string
	decNum  bool
	skip    string // why the differential tests are skipped
}

var (
	jqOnce sync.Once
	jqBin  jqBinary
)

func findJQ() jqBinary {
	jqOnce.Do(func() {
		path, err := exec.LookPath("jq")
		if err != nil {
			jqBin.skip = "jq is not installed"
			return
		}
		jqBin.path = path
		out, err := exec.Command(path, "--version").Output()
		if err != nil {
			jqBin.skip = "jq --version failed: " + err.Error()
			return
		}
		jqBin.version = strings.TrimSpace(string(out))
		m := regexp.MustCompile(`^jq-(\d+)\.(\d+)`).FindStringSubmatch(jqBin.version)
		if m == nil {
			jqBin.skip = "unrecognized jq version " + jqBin.version
			return
		}
		major, _ := strconv.Atoi(m[1])
		minor, _ := strconv.Atoi(m[2])
		if major < 1 || (major == 1 && minor < 7) {
			jqBin.skip = jqBin.version + " is older than jq 1.7"
			return
		}
		out, err = exec.Command(path, "-n", "1.0").Output()
		jqBin.decNum = err == nil && string(out) == "1.0\n"
	})
	return jqBin
}

// requireJQ skips t unless a suitable jq is installed.
func requireJQ(t testing.TB) jqBinary {
	t.Helper()
	bin := findJQ()
	if bin.skip != "" {
		t.Skip(bin.skip)
	}
	return bin
}

// runJQ runs jq -r filter on input. ok is false when jq failed (a compile
// or runtime error); out holds what it printed to stdout before that.
func runJQ(t testing.TB, bin jqBinary, filter, input string) (out string, ok bool, stderr string) {
	t.Helper()
	cmd := exec.Command(bin.path, "-r", filter)
	cmd.Stdin = strings.NewReader(input)
	var stdout, errOut bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &errOut
	err := cmd.Run()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatalf("running jq: %v", err)
	}
	return stdout.String(), err == nil, errOut.String()
}

// runClix runs the filter with this package. parseErr is set when Parse
// failed.
func runClix(filter, input string) (out string, parseErr, runErr error) {
	q, err := Parse(filter)
	if err != nil {
		return "", err, nil
	}
	var b bytes.Buffer
	err = q.Write(&b, []byte(input))
	return b.String(), nil, err
}
