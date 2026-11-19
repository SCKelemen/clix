package clix

import "context"

// MainForTest runs a.main, Main without the process exit, for the
// external tests.
func MainForTest(a *App, args []string) int { return a.main(context.Background(), args) }
