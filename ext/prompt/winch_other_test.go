//go:build !unix

package prompt_test

import "testing"

func sendWinch(t *testing.T) { t.Skip("no SIGWINCH on this platform") }
