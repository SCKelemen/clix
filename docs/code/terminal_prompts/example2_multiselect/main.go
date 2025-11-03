package main

import (
	"context"
	"fmt"
	"os"

	"clix"
	"clix/ext/prompt"
)

func main() {
	app := clix.NewApp("demo")
	app.Out = os.Stdout
	app.In = os.Stdin

	// Add Terminal Prompt extension
	app.AddExtension(prompt.Extension{})

	// Apply extensions (required before Run)
	if err := app.ApplyExtensions(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cmd := clix.NewCommand("demo")
	cmd.Run = func(ctx *clix.Context) error {
		choices, err := ctx.App.Prompter.Prompt(context.Background(), clix.PromptRequest{
			Label: "Select languages",
			Options: []clix.SelectOption{
				{Label: "Go", Value: "go"},
				{Label: "Python", Value: "python"},
				{Label: "JavaScript", Value: "js"},
				{Label: "Rust", Value: "rust"},
			},
			MultiSelect: true,
			Theme:      ctx.App.DefaultTheme,
		})
		if err != nil {
			return err
		}

		fmt.Fprintf(ctx.App.Out, "Selected: %s\n", choices)
		return nil
	}

	app.Root = cmd

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

