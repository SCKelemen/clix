package config

import (
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// config set stores only values the ISO 8601 flags accept, as for any
// other flag.
func TestConfigE2E_ISOFlags(t *testing.T) {
	app, out, _ := e2eApp(t, "")
	app.Flags().TimeVar(clix.TimeVarOptions{FlagOptions: clix.FlagOptions{Name: "since", Usage: "Since"}})
	app.Flags().ISODurationVar(clix.ISODurationVarOptions{FlagOptions: clix.FlagOptions{Name: "every", Usage: "Every"}})
	app.Flags().DurationVar(clix.DurationVarOptions{FlagOptions: clix.FlagOptions{Name: "timeout", Usage: "Timeout"}})

	for key, value := range map[string]string{"since": "2026-11-19", "every": "P1M", "timeout": "PT5M"} {
		if got := run(t, app, out, "config", "set", key, value); got != key+" = "+value+"\n" {
			t.Errorf("config set %s %s printed %q", key, value, got)
		}
	}
	for _, tc := range []struct{ key, value, msg string }{
		{"since", "2026-13-01", "month 13 is out of range"},
		{"every", "1h", `iso8601: invalid duration "1h"`},
		{"timeout", "P1M", "years or months"},
	} {
		if _, err := tryRun(app, out, "config", "set", tc.key, tc.value); err == nil || !strings.Contains(err.Error(), tc.msg) {
			t.Errorf("config set %s %s: %v, want an error containing %q", tc.key, tc.value, err, tc.msg)
		}
	}
}
