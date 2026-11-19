package prompt_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/SCKelemen/clix/v2/ext/prompt"
	"github.com/SCKelemen/clix/v2/internal/termprobe"
)

// liveHelperEnv makes the test binary run as a helper program inside a
// real terminal instead of running the tests: the key probe ("probe") or
// the demo app ("demo"). The live terminal tests start it that way (see
// live_terminals_test.go).
const liveHelperEnv = "CLIX_LIVE_HELPER"

func TestMain(m *testing.M) {
	switch os.Getenv(liveHelperEnv) {
	case "probe":
		os.Exit(runProbeHelper())
	case "demo":
		os.Exit(runDemoHelper())
	}
	os.Exit(m.Run())
}

// runProbeHelper records key sequences with termprobe, set up exactly as a
// raw-mode prompt sets up the terminal (prompts draw on standard error).
func runProbeHelper() int {
	err := termprobe.Run(os.Stdin, os.Stderr, termprobe.Config{
		Mode:   os.Getenv("CLIX_PROBE_MODE"),
		Title:  os.Getenv("CLIX_PROBE_TITLE"),
		Output: os.Getenv("CLIX_PROBE_OUT"),
		Setup:  func() (func(), error) { return prompt.RawSetupForTest(os.Stdin, os.Stderr) },
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "probe:", err)
		return 1
	}
	return 0
}
