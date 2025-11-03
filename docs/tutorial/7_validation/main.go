package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"

	"clix"
)

func main() {
	app := clix.NewApp("demo")
	app.Out = os.Stdout
	app.In = os.Stdin

	cmd := clix.NewCommand("age")
	cmd.Arguments = []*clix.Argument{
		{
			Name:     "age",
			Prompt:   "Your age",
			Required: true,
			Validate: func(value string) error {
				age, err := strconv.Atoi(value)
				if err != nil {
					return errors.New("age must be a number")
				}
				if age < 0 {
					return errors.New("age cannot be negative")
				}
				if age > 150 {
					return errors.New("age cannot be greater than 150")
				}
				return nil
			},
		},
	}

	cmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Your age is %s\n", ctx.Args[0])
		return nil
	}

	app.Root = cmd

	if err := app.Run(context.Background(), nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
