package simple

import (
	"fmt"
	"strings"

	"clix"
)

func NewCommand(name, description string) *clix.Command {
	cmd := clix.NewCommand(name)
	cmd.Short = description
	cmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "%s: %s\n", strings.ToUpper(name), description)
		return nil
	}
	return cmd
}
