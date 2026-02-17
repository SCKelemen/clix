package greet

import (
	"fmt"

	"github.com/SCKelemen/clix/v2"
)

func NewCommand(project *string) *clix.Command {
	cmd := clix.NewCommand("greet")
	cmd.Short = "Print a greeting"
	cmd.Usage = "demo greet [flags] <name>"

	var name string
	cmd.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:       "name",
			Usage:      "Name of the person to greet",
			Required:   true,
			Prompt:     "Name of the person to greet",
			Positional: true,
		},
		Value: &name,
	})

	var salutation string
	cmd.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:  "salutation",
			Short: "s",
			Usage: "Salutation prefix to use",
		},
		Default: "Hello",
		Value:   &salutation,
	})

	cmd.PreRun = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Using project %s\n", valueOrDefault(project, "sample-project"))
		return nil
	}

	cmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "%s %s!\n", salutation, name)
		return nil
	}

	cmd.PostRun = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "All done!")
		return nil
	}

	return cmd
}

func valueOrDefault(value *string, fallback string) string {
	if value != nil && *value != "" {
		return *value
	}
	return fallback
}
