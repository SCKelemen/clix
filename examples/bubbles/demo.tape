Output demo.gif
Set Width 1200
Set Height 800
Set Theme "Monokai Pro"
Set FontSize 18
Set Shell "bash"
Env TERM "xterm-256color"

# Build the example
Type "go build -o demo ./cmd/demo"
Enter
Sleep 2s

# Show help
Type "./demo"
Enter
Sleep 1s

Type "./demo greet"
Enter
Sleep 500ms
Type "Alice"
Enter
Sleep 500ms
Type "\033[B"
Sleep 300ms
Enter
Sleep 500ms
Type "y"
Enter
Sleep 1s
