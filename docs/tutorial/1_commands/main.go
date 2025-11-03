package main

import (
	"context"
	"fmt"
	"os"

	"clix"
)

func main() {
	app := clix.NewApp("greet")
	app.Out = os.Stdout
	app.In = os.Stdin

	// Create the root command
	greetCmd := clix.NewCommand("greet")
	greetCmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "Hello, World!")
		return nil
	}

	app.Root = greetCmd

	ctx := context.Background()
	if err := app.Run(ctx, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

