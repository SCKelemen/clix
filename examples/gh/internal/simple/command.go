package simple

import (
	"fmt"
	"strings"

	"github.com/SCKelemen/clix/v2"
)

func NewCommand(name, description string) *clix.Command {
	cmd := clix.NewCommand(name)
	cmd.Short = description
	cmd.Run = func(ctx *clix.Context) error {
		_, err := fmt.Fprintf(ctx.App.Out, "%s: %s\n", strings.ToUpper(name), description)
		return err
	}
	return cmd
}
