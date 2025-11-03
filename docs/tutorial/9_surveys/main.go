package main

import (
	"context"
	"fmt"
	"os"

	"clix"
	"clix/ext/survey"
)

func main() {
	app := clix.NewApp("demo")
	app.Out = os.Stdout
	app.In = os.Stdin

	// Add survey extension
	app.AddExtension(survey.Extension{})

	// Apply extensions (required before Run)
	if err := app.ApplyExtensions(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cmd := clix.NewCommand("survey")
	cmd.Run = func(ctx *clix.Context) error {
		s := survey.New(context.Background(), ctx.App.Prompter)

		// Add questions dynamically
		s.Ask(clix.PromptRequest{
			Label: "What is your name?",
			Theme: ctx.App.DefaultTheme,
		}, func(answer string, s *survey.Survey) {
			// Handler receives answer and can add more questions
			fmt.Fprintf(ctx.App.Out, "Hello, %s!\n", answer)

			// Add a follow-up question
			s.Ask(clix.PromptRequest{
				Label:   "Would you like to continue?",
				Confirm: true,
				Theme:   ctx.App.DefaultTheme,
			}, func(answer2 string, s *survey.Survey) {
				if answer2 == "y" {
					fmt.Fprintln(ctx.App.Out, "Great! Continuing...")
				} else {
					fmt.Fprintln(ctx.App.Out, "Okay, stopping.")
				}
			})
		})

		// Run the survey
		return s.Run()
	}

	app.Root = cmd

	ctx := context.Background()
	if err := app.Run(ctx, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

