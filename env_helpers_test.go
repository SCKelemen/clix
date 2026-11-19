package clix

import (
	"os"
	"testing"
)

// unsetenv unsets key for the rest of the test. t.Setenv records the
// variable's current value first, so it is restored when the test ends.
func unsetenv(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
}
