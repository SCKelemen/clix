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

	// Root command
	rootCmd := clix.NewCommand("greet")
	rootCmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "Usage: greet <command>")
		return nil
	}

	// Subcommand: greet hello
	helloCmd := clix.NewCommand("hello")
	helloCmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "Hello!")
		return nil
	}

	// Subcommand: greet goodbye
	goodbyeCmd := clix.NewCommand("goodbye")
	goodbyeCmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "Goodbye!")
		return nil
	}

	// Add subcommands to root
	rootCmd.AddSubcommand(helloCmd)
	rootCmd.AddSubcommand(goodbyeCmd)

	app.Root = rootCmd

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
