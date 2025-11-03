package main

import (
	"fmt"
	"os"

	"clix"
)

func main() {
	app := clix.NewApp("greet")
	app.Out = os.Stdout
	app.In = os.Stdin

	greetCmd := clix.NewCommand("greet")
	greetCmd.Arguments = []*clix.Argument{
		{
			Name:     "name",
			Prompt:   "What is your name?",
			Required: true,
		},
	}

	greetCmd.Run = func(ctx *clix.Context) error {
		name := ctx.Args[0]
		fmt.Fprintf(ctx.App.Out, "Hello, %s!\n", name)
		return nil
	}

	app.Root = greetCmd

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

