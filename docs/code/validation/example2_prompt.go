package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"clix"
)

func main() {
	app := clix.NewApp("demo")
	app.Out = os.Stdout
	app.In = os.Stdin

	cmd := clix.NewCommand("demo")
	cmd.Run = func(ctx *clix.Context) error {
		email, err := ctx.App.Prompter.Prompt(context.Background(), clix.PromptRequest{
			Label: "Email address",
			Validate: func(value string) error {
				if !strings.Contains(value, "@") {
					return errors.New("email must contain @")
				}
				return nil
			},
			Theme: ctx.App.DefaultTheme,
		})
		if err != nil {
			return err
		}

		fmt.Fprintf(ctx.App.Out, "Email: %s\n", email)
		return nil
	}

	app.Root = cmd

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

