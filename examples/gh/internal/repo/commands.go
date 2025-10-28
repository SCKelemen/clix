package repo

import (
	"fmt"

	"clix"
)

func NewCommand() *clix.Command {
	cmd := clix.NewCommand("repo")
	cmd.Short = "Manage repositories"

	clone := clix.NewCommand("clone")
	clone.Short = "Clone a repository"
	clone.Arguments = []*clix.Argument{{Name: "repository", Prompt: "OWNER/REPO", Required: true}}
	clone.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Cloning %s...\n", ctx.Args[0])
		return nil
	}

	create := clix.NewCommand("create")
	create.Short = "Create a new repository"
	create.Arguments = []*clix.Argument{{Name: "name", Prompt: "Repository name", Required: true}}

	var visibility string
	create.Flags.StringVar(&clix.StringVarOptions{
		Name:    "visibility",
		Usage:   "Repository visibility (public, private)",
		Default: "public",
		Value:   &visibility,
	})
	create.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Creating %s repository %s\n", visibility, ctx.Args[0])
		return nil
	}

	cmd.AddCommand(clone)
	cmd.AddCommand(create)
	return cmd
}
