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

	var name string
	var age int

	greetCmd := clix.NewCommand("greet")
	greetCmd.Flags.StringVar(&clix.StringVarOptions{
		Name:  "name",
		Short: "n",
		Usage: "Your name",
	}, &name)

	greetCmd.Flags.IntVar(&clix.IntVarOptions{
		Name:  "age",
		Short: "a",
		Usage: "Your age",
	}, &age)

	greetCmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Hello, %s! You are %d years old.\n", name, age)
		return nil
	}

	app.Root = greetCmd

	if err := app.Run(context.Background(), nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

