package main

import (
	"context"
	"fmt"
	"os"

	"clix"
)

func main() {
	app := clix.NewApp("demo")
	app.Out = os.Stdout
	app.In = os.Stdin

	cmd := clix.NewCommand("greet")
	cmd.Run = func(ctx *clix.Context) error {
		// Prompt for name
		name, err := ctx.App.Prompter.Prompt(context.Background(), clix.PromptRequest{
			Label: "What is your name?",
			Theme: ctx.App.DefaultTheme,
		})
		if err != nil {
			return err
		}

		fmt.Fprintf(ctx.App.Out, "Hello, %s!\n", name)
		return nil
	}

	app.Root = cmd

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

