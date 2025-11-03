package main

import (
	"context"
	"fmt"
	"os"

	"clix"
)

// MyExtension is a custom extension that adds a "greet" command
type MyExtension struct{}

// Extend implements clix.Extension
func (e MyExtension) Extend(app *clix.App) error {
	if app.Root == nil {
		return nil
	}

	// Add a custom command to the root
	greetCmd := clix.NewCommand("greet")
	greetCmd.Short = "A custom greeting command added by extension"
	greetCmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "Hello from my custom extension!")
		return nil
	}

	app.Root.AddCommand(greetCmd)
	return nil
}

func main() {
	app := clix.NewApp("demo")
	app.Out = os.Stdout
	app.In = os.Stdin

	// Add custom extension
	app.AddExtension(MyExtension{})

	// Apply extensions (required before Run)
	if err := app.ApplyExtensions(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Root command
	rootCmd := clix.NewCommand("demo")
	rootCmd.Short = "Demo application with custom extension"
	rootCmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "This app has a custom 'greet' command added by an extension!")
		return nil
	}

	app.Root = rootCmd

	ctx := context.Background()
	if err := app.Run(ctx, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

