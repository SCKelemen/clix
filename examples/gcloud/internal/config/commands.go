package config

import (
	"fmt"
	"strings"

	"github.com/SCKelemen/clix/v2"
)

func NewCommand(project *string) *clix.Command {
	cmd := clix.NewCommand("config")
	cmd.Short = "View and edit Google Cloud CLI properties"
	cmd.Run = func(ctx *clix.Context) error {
		return clix.HelpRenderer{App: ctx.App, Command: ctx.Command}.Render(ctx.App.Out)
	}

	set := clix.NewCommand("set")
	set.Short = "Set a property"

	var setProp, setValue string
	set.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:       "property",
			Usage:      "Property name",
			Required:   true,
			Prompt:     "Property name",
			Positional: true,
		},
		Value: &setProp,
	})
	set.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:       "value",
			Usage:      "Property value",
			Required:   true,
			Prompt:     "Value",
			Positional: true,
		},
		Value: &setValue,
	})
	set.Run = func(ctx *clix.Context) error {
		if strings.EqualFold(setProp, "project") {
			*project = setValue
		}
		fmt.Fprintf(ctx.App.Out, "Set %s to %s\n", setProp, setValue)
		return nil
	}

	get := clix.NewCommand("get")
	get.Short = "Get a property"

	var getProp string
	get.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:       "property",
			Usage:      "Property name",
			Required:   true,
			Prompt:     "Property name",
			Positional: true,
		},
		Value: &getProp,
	})
	get.Run = func(ctx *clix.Context) error {
		if strings.EqualFold(getProp, "project") {
			fmt.Fprintf(ctx.App.Out, "project = %s\n", valueOrDefault(project, ""))
			return nil
		}
		fmt.Fprintf(ctx.App.Out, "%s is not set\n", getProp)
		return nil
	}
	cmd.Children = []*clix.Command{
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
