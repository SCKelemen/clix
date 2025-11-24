// Package clix provides a declarative framework for building command line
// interfaces with nested commands, global flags, configuration hydration and
// interactive prompting. The package exposes high level types such as App and
// Command which can be composed to describe the command tree and execution
// behaviour of a CLI.
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
//     Args, hydrated flags/config).
//
//   - Within command handlers, pass *clix.Context to any functions that need
//     CLI metadata. Because *clix.Context embeds context.Context, you can
//     pass it anywhere a context.Context is required (e.g., to Prompter.Prompt).
//
//   - Internal clix functions that need CLI awareness (App, Command, Args)
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
//		// Use ctx.App, ctx.Command, ctx.Args for CLI data
//		return nil
//	}
package clix
