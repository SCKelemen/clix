Output styled.gif
Set Width 1200
Set Height 800
Set Theme "Monokai Pro"
Set FontSize 18
Set Shell "bash"
Env TERM "xterm-256color"

# Build the example
Type "go build -o styled ./cmd/styled"
Enter
Sleep 2s

# Run styled (prompts for name)
Type "./styled"
Enter
Sleep 2s
# Wait for banner and name prompt to appear, then type name
Type "Test User"
Enter
Sleep 2s
# Command completes and shows output, wait for it to finish
Sleep 1s

# Show help
Type "./styled --help"
Enter
Sleep 2s

# Show format option (non-interactive)
Type "./styled style --format json"
Enter
Sleep 1s
