package pr

import (
	"fmt"

	"github.com/SCKelemen/clix"
)

func NewCommand() *clix.Command {
	cmd := clix.NewCommand("pr")
	cmd.Short = "Manage pull requests"
	cmd.Run = func(ctx *clix.Context) error {
		return clix.HelpRenderer{App: ctx.App, Command: ctx.Command}.Render(ctx.App.Out)
	}

	checkout := clix.NewCommand("checkout")
	checkout.Short = "Check out a pull request"
	checkout.Aliases = []string{"co"}
	checkout.Arguments = []*clix.Argument{{Name: "number", Prompt: "Pull request number", Required: true}}
	checkout.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Checking out PR #%s\n", ctx.Args[0])
		return nil
	}

	merge := clix.NewCommand("merge")
	merge.Short = "Merge a pull request"
	merge.Arguments = []*clix.Argument{{Name: "number", Prompt: "Pull request number", Required: true}}

	var rebase bool
	merge.Flags.BoolVar(clix.BoolVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:  "rebase",
			Usage: "Rebase the branch before merging",
		},
		Value: &rebase,
	})

	merge.Run = func(ctx *clix.Context) error {
		strategy := "merge commit"
		if rebase {
			strategy = "rebase"
		}
		fmt.Fprintf(ctx.App.Out, "Merging PR #%s using %s strategy\n", ctx.Args[0], strategy)
		return nil
	}
	cmd.Children = []*clix.Command{
		checkout,
		merge,
	}
	return cmd
}
