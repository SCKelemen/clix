package main

import (
	"context"
	"fmt"
	"os"

	"clix"
	"clix/ext/help"
)

func main() {
	app := clix.NewApp("greet")
	app.Out = os.Stdout
	app.In = os.Stdin

	// Add help extension (adds "help" command)
	app.AddExtension(help.Extension{})

	// Apply extensions (required before Run)
	if err := app.ApplyExtensions(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Root command
	rootCmd := clix.NewCommand("greet")
	rootCmd.Short = "A greeting application"
	rootCmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "Usage: greet <command>")
		return nil
	}

	// Subcommand: greet hello
	helloCmd := clix.NewCommand("hello")
	helloCmd.Short = "Say hello"
	helloCmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "Hello!")
		return nil
	}

	// Subcommand: greet goodbye
	goodbyeCmd := clix.NewCommand("goodbye")
	goodbyeCmd.Short = "Say goodbye"
	goodbyeCmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "Goodbye!")
		return nil
	}

	// Add subcommands to root
	rootCmd.Subcommands = []*clix.Command{helloCmd, goodbyeCmd}

	app.Root = rootCmd

	ctx := context.Background()
	if err := app.Run(ctx, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

