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

	cmd := clix.NewCommand("server")
	cmd.Run = func(ctx *clix.Context) error {
		port, err := ctx.App.Prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:   "Port number",
			Default: "8080",
			Theme:   ctx.App.DefaultTheme,
		})
		if err != nil {
			return err
		}

		fmt.Fprintf(ctx.App.Out, "Using port: %s\n", port)
		return nil
	}

	app.Root = cmd

	if err := app.Run(context.Background(), nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

