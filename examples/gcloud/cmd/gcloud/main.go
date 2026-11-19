// Command gcloud is a Google Cloud CLI-style example of clix: named
// configurations, a global Ambient --project flag, a flag group, help
// topics and a progress indicator for a long-running operation.
package main

import "context"

func main() {
	// Main runs the app with os.Args[1:], cancels the context on Ctrl-C,
	// prints any error and exits with clix.ExitCode(err).
	newApp().Main(context.Background())
}
