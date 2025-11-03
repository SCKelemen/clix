package main

import (
	"fmt"
	"os"

	"clix"
)

func main() {
	app := clix.NewApp("myapp")
	app.Out = os.Stdout
	app.In = os.Stdin

	var apiKey string
	var port int

	cmd := clix.NewCommand("server")
	cmd.Flags.StringVar(&clix.StringVarOptions{
		Name:   "api-key",
		Usage:  "API key for authentication",
		EnvVar: "MYAPP_API_KEY", // Reads from environment
	}, &apiKey)

	cmd.Flags.IntVar(&clix.IntVarOptions{
		Name:    "port",
		Usage:   "Server port",
		Default: "8080",
		EnvVar:  "MYAPP_PORT",
	}, &port)

	cmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "API Key: %s\n", apiKey)
		fmt.Fprintf(ctx.App.Out, "Port: %d\n", port)
		return nil
	}

	app.Root = cmd

	if err := app.Run(context.Background(), nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

