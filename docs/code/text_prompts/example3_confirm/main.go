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

	cmd := clix.NewCommand("demo")
	cmd.Run = func(ctx *clix.Context) error {
		confirmed, err := ctx.App.Prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:   "Continue?",
			Confirm: true,
			Default: "y",
			Theme:   ctx.App.DefaultTheme,
		})
		if err != nil {
			return err
		}

		if confirmed == "y" {
			fmt.Fprintln(ctx.App.Out, "Proceeding...")
		} else {
			fmt.Fprintln(ctx.App.Out, "Cancelled.")
		}
		return nil
	}

	app.Root = cmd

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

