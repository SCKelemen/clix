package config

import (
	"fmt"
	"strings"

	"clix"
)

func NewCommand(project *string) *clix.Command {
	cmd := clix.NewCommand("config")
	cmd.Short = "View and edit Google Cloud CLI properties"
	cmd.Run = func(ctx *clix.Context) error {
		return clix.HelpRenderer{App: ctx.App, Command: ctx.Command}.Render(ctx.App.Out)
	}

	set := clix.NewCommand("set")
	set.Short = "Set a property"
	set.Arguments = []*clix.Argument{
		{Name: "property", Prompt: "Property name", Required: true},
		{Name: "value", Prompt: "Value", Required: true},
	}
	set.Run = func(ctx *clix.Context) error {
		if strings.EqualFold(ctx.Args[0], "project") {
			*project = ctx.Args[1]
		}
		fmt.Fprintf(ctx.App.Out, "Set %s to %s\n", ctx.Args[0], ctx.Args[1])
		return nil
	}

	get := clix.NewCommand("get")
	get.Short = "Get a property"
	get.Arguments = []*clix.Argument{{Name: "property", Prompt: "Property name", Required: true}}
	get.Run = func(ctx *clix.Context) error {
		if strings.EqualFold(ctx.Args[0], "project") {
			fmt.Fprintf(ctx.App.Out, "project = %s\n", valueOrDefault(project, ""))
			return nil
		}
		fmt.Fprintf(ctx.App.Out, "%s is not set\n", ctx.Args[0])
		return nil
	}
	cmd.Subcommands = []*clix.Command{
		set,
		get,
	}
	return cmd
}

func valueOrDefault(value *string, fallback string) string {
	if value != nil && *value != "" {
		return *value
	}
	return fallback
}
