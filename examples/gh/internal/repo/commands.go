package repo

import (
	"fmt"

	"github.com/SCKelemen/clix/v2"
)

func NewCommand() *clix.Command {
	cmd := clix.NewCommand("repo")
	cmd.Short = "Manage repositories"
	cmd.Run = func(ctx *clix.Context) error {
		return clix.HelpRenderer{App: ctx.App, Command: ctx.Command}.Render(ctx.App.Out)
	}

	clone := clix.NewCommand("clone")
	clone.Short = "Clone a repository"

	var repository string
	clone.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:       "repository",
			Usage:      "Repository to clone (OWNER/REPO)",
			Required:   true,
			Prompt:     "OWNER/REPO",
			Positional: true,
		},
		Value: &repository,
	})
	clone.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Cloning %s...\n", repository)
		return nil
	}

	create := clix.NewCommand("create")
	create.Short = "Create a new repository"

	var repoName string
	create.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:       "name",
			Usage:      "Repository name",
			Required:   true,
			Prompt:     "Repository name",
			Positional: true,
		},
		Value: &repoName,
	})

	var visibility string
	create.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:  "visibility",
			Usage: "Repository visibility (public, private)",
		},
		Default: "public",
		Value:   &visibility,
	})
	create.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Creating %s repository %s\n", visibility, repoName)
		return nil
	}
	cmd.Children = []*clix.Command{
		clone,
		create,
	}
	return cmd
}
