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

# Show styled help
Type "./styled"
Enter
Sleep 1s

Type "./styled --help"
Enter
Sleep 2s

# Run style command with interactive prompt
Type "./styled style"
Enter
Sleep 1.5s
# Wait for name prompt to appear, then type name
Type "Test User"
Enter
Sleep 1.5s
# Wait for confirmation prompt
Type "y"
Enter
Sleep 1.5s

# Show format option (non-interactive)
Type "./styled style --format json"
Enter
Sleep 1s
