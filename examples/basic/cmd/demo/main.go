package main

import (
	"context"

	demoapp "github.com/SCKelemen/clix/v2/examples/basic/internal/app"
)

func main() {
	// Main runs the app with os.Args[1:], cancels the context on Ctrl-C,
	// prints any error and exits with clix.ExitCode(err).
	demoapp.New().Main(context.Background())
}
