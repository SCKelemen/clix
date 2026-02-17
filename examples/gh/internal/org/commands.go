package org

import (
	"fmt"

	"github.com/SCKelemen/clix/v2"
)

func NewCommand() *clix.Command {
	cmd := clix.NewCommand("org")
	cmd.Short = "Manage organizations"
	cmd.Run = func(ctx *clix.Context) error {
		return clix.HelpRenderer{App: ctx.App, Command: ctx.Command}.Render(ctx.App.Out)
	}

	list := clix.NewCommand("list")
	list.Short = "List accessible organizations"
	list.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "- cli")
		fmt.Fprintln(ctx.App.Out, "- octo-org")
		return nil
	}

	view := clix.NewCommand("view")
	view.Short = "Show organization details"

	var organization string
	view.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:       "organization",
			Usage:      "Organization login",
			Required:   true,
			Prompt:     "Organization login",
			Positional: true,
		},
		Value: &organization,
	})
	view.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Organization: %s\n", organization)
		return nil
	}
	cmd.Children = []*clix.Command{
		list,
		view,
	}
	return cmd
}
