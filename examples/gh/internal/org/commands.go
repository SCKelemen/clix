package org

import (
	"fmt"

	"clix"
)

func NewCommand() *clix.Command {
	cmd := clix.NewCommand("org")
	cmd.Short = "Manage organizations"

	list := clix.NewCommand("list")
	list.Short = "List accessible organizations"
	list.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "- cli")
		fmt.Fprintln(ctx.App.Out, "- octo-org")
		return nil
	}

	view := clix.NewCommand("view")
	view.Short = "Show organization details"
	view.Arguments = []*clix.Argument{{Name: "organization", Prompt: "Organization login", Required: true}}
	view.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Organization: %s\n", ctx.Args[0])
		return nil
	}
	cmd.Subcommands = []*clix.Command{
		list,
		view,
	}
	return cmd
}
