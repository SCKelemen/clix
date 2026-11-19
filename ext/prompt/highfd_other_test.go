//go:build !(darwin || linux)

package prompt_test

import (
	"os"
	"testing"

	"github.com/SCKelemen/clix/v2/internal/ptytest"
)

func highFd(t *testing.T, term *ptytest.Term) *os.File {
	t.Skip("needs dup2")
	return nil
}
