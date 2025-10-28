package demo

import (
	"fmt"

	"clix"
)

// NewApp constructs the demo CLI application with commands, flags, and configuration.
func NewApp() *clix.App {
	app := clix.NewApp("demo")
	app.Description = "Demonstrates the clix CLI framework"

	var project string
	app.GlobalFlags.StringVar(&clix.StringVarOptions{
		Name:    "project",
		Usage:   "Project to operate on",
		EnvVar:  "DEMO_PROJECT",
		Value:   &project,
		Default: "sample-project",
	})

	root := clix.NewCommand("demo")
	root.Short = "Root of the demo application"

	greet := clix.NewCommand("greet")
	greet.Short = "Print a greeting"
	greet.Usage = "demo greet [name]"
	greet.Arguments = []*clix.Argument{{
		Name:     "name",
		Prompt:   "Name of the person to greet",
		Required: true,
	}}

	var salutation string
	greet.Flags.StringVar(&clix.StringVarOptions{
		Name:    "salutation",
		Short:   "s",
		Usage:   "Salutation prefix to use",
		Default: "Hello",
		Value:   &salutation,
	})

	greet.PreRun = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Using project %s\n", project)
		return nil
	}

	greet.Run = func(ctx *clix.Context) error {
		name := ctx.Args[0]
		fmt.Fprintf(ctx.App.Out, "%s %s!\n", salutation, name)
		return nil
	}

	greet.PostRun = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "All done!")
		return nil
	}

	root.AddCommand(greet)
	app.Root = root

	return app
}
