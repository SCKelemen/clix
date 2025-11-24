package app

import (
	"fmt"
	"strings"

	"clix"
	"clix/examples/basic/internal/greet"
)

const demoBanner = `
  ██████╗ ██╗      ██╗ ██╗  ██╗
 ██╔════╝ ██║      ██║ ╚██╗██╔╝
 ██║      ██║      ██║  ╚███╔╝
 ██║      ██║      ██║  ██╔██╗
 ╚██████╗ ███████╗ ██║ ██╔╝ ██╗
  ╚═════╝ ╚══════╝ ╚═╝ ╚═╝  ╚═╝
`

// New returns a configured application that demonstrates the clix framework.
func New() *clix.App {
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
	root.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, strings.Trim(demoBanner, "\n"))
		return clix.HelpRenderer{App: ctx.App, Command: ctx.Command}.Render(ctx.App.Out)
	}
	root.Children = []*clix.Command{
		greet.NewCommand(&project),
	}

	app.Root = root
	return app
}
