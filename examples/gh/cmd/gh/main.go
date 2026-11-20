// Command gh is a GitHub CLI-style example of clix: tables, gh's --json,
// --jq and --template, aliases and plugins.
package main

import "context"

func main() {
	// Main runs the app with os.Args[1:], cancels the context on Ctrl-C,
	// prints any error and exits with clix.ExitCode(err), or with a
	// plugin's own exit status.
	newApp().Main(context.Background())
}
