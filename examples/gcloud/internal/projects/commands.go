package projects

import (
	"fmt"

	"clix"
)

func NewCommand(project *string) *clix.Command {
	cmd := clix.NewCommand("projects")
	cmd.Short = "Create and manage project access policies"
	cmd.Run = func(ctx *clix.Context) error {
		return clix.HelpRenderer{App: ctx.App, Command: ctx.Command}.Render(ctx.App.Out)
	}

	list := clix.NewCommand("list")
	list.Short = "List projects"
	list.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "PROJECT_ID            NAME")
		fmt.Fprintf(ctx.App.Out, "%s            Sample Project\n", valueOrDefault(project, "demo-project"))
		return nil
	}

	create := clix.NewCommand("create")
	create.Short = "Create a project"
	create.Arguments = []*clix.Argument{{Name: "project-id", Prompt: "New project ID", Required: true}}
	create.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Creating project %s\n", ctx.Args[0])
		return nil
	}
	cmd.Subcommands = []*clix.Command{
		list,
		create,
	}
	return cmd
}

func valueOrDefault(value *string, fallback string) string {
	if value != nil && *value != "" {
		return *value
	}
	return fallback
}
