//go:build !(darwin || linux)

package autocomplete

import "testing"

// zshListsFlags needs a pty, which the tests only open on macOS and
// Linux.
func zshListsFlags(t *testing.T) {
	t.Log("no pty on this platform; TestShell_Zsh_* cover a real zsh in CI")
}
