package main

import (
	"clix"
	"clix/examples/basic/internal/greet"
)

func newApp() *clix.App {
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
	root.Subcommands = []*clix.Command{
		greet.NewCommand(&project),
	}

	app.Root = root
	return app
}
