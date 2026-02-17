// Package clix provides a declarative framework for building command line
// interfaces with nested commands, named flags, configuration hydration and
// interactive prompting. The package exposes high level types such as App and
// Command which can be composed to describe the command tree and execution
// behavior of a CLI.
//
// # Getting Started
//
// Define an App, create commands with handlers, and call Run:
//
//	app := clix.NewApp("myapp")
//	root := clix.NewCommand("myapp")
//
//	greet := clix.NewCommand("greet")
//	greet.Short = "Greet someone"
//
//	var name string
//	greet.Flags.StringVar(clix.StringVarOptions{
//		FlagOptions: clix.FlagOptions{
//			Name:     "name",
//			Usage:    "Name of the person to greet",
//			Required: true,
//			Prompt:   "Name",
//		},
//		Value: &name,
//	})
//	greet.Run = func(ctx *clix.Context) error {
//		fmt.Printf("Hello, %s!\n", name)
//		return nil
//	}
//
//	root.AddCommand(greet)
//	app.Root = root
//	app.Run(context.Background(), nil)
//
// # Core Types
//
// The primary stable types for v2 are:
//
//   - App – owns the root command, flags, and extensions
//   - Command – represents a node in the CLI tree (group or command)
//   - Context – wraps context.Context with CLI metadata (App, Command)
//   - Extension – plugs cross-cutting behavior into App
//   - FlagSet / Flag – named parameters with Required/Prompt support
//
// # v2 Changes
//
// In v2, positional arguments have been removed. All parameters are named flags:
//
//   - Use Required: true on FlagOptions to mark a flag as mandatory
//   - Use Prompt: "label" to set the interactive prompt label
//   - Three execution modes:
//     1. No flags passed → interactive prompting for required flags
//     2. All required flags satisfied → run
//     3. Some flags passed but required missing → error
//
// # Context Usage
//
// clix uses a layered context design:
//
//   - App.Run(ctx context.Context, args []string) accepts a standard
//     context.Context that controls process-level cancellation and deadlines.
//
//   - For each command execution, clix builds a *clix.Context which embeds
//     the original context.Context and adds CLI-specific data (App, Command,
//     hydrated flags/config).
//
//   - Within command handlers, pass *clix.Context to any functions that need
//     CLI metadata. Because *clix.Context embeds context.Context, you can
//     pass it anywhere a context.Context is required (e.g., to Prompter.Prompt).
//
//   - Internal clix functions that need CLI awareness (App, Command)
//     should accept *clix.Context. Functions that only need cancellation/deadlines
//     should accept context.Context.
//
// Example:
//
//	cmd.Run = func(ctx *clix.Context) error {
//		// ctx is both a context.Context (for cancellation) and has CLI data
//		value, err := ctx.App.Prompter.Prompt(ctx, clix.PromptRequest{
//			Label: "Enter value",
//		})
//		// Use ctx.Done(), ctx.Err() for cancellation
//		// Use ctx.App, ctx.Command for CLI data
//		return nil
//	}
//
// # Compatibility
//
// clix v2 follows semantic versioning. The github.com/SCKelemen/clix/v2 module path
// will not have breaking changes within major version 2. Extensions under
// github.com/SCKelemen/clix/v2/ext/... may evolve more quickly but will also respect
// semver for their exported APIs.
//
// clix requires Go 1.24 or later.
package clix
