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
			Name:     "first-name",
			Prompt:   "First name",
			Required: true,
		},
		{
			Name:     "last-name",
			Prompt:   "Last name",
			Required: true,
		},
	}

	greetCmd.Run = func(ctx *clix.Context) error {
		firstName := ctx.Args[0]
		lastName := ctx.Args[1]
		fmt.Fprintf(ctx.App.Out, "Hello, %s %s!\n", firstName, lastName)
		return nil
	}

	app.Root = greetCmd

	if err := app.Run(context.Background(), nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

