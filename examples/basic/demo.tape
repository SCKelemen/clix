Output demo.gif
Set Width 1200
Set Height 800
Set Theme "Monokai Pro"
Set FontSize 18
Set Shell "bash"

# Debug: Print working directory
Type "pwd"
Enter
Sleep 500ms

# Build the example
Type "go build -o demo ./cmd/demo"
Enter
Sleep 2s

# Show help
Type "./demo"
Enter
Sleep 1s

Type "./demo --help"
Enter
Sleep 1.5s

Type "./demo greet --help"
Enter
Sleep 1.5s

Type "./demo greet Alice"
Enter
Sleep 1s

Type "./demo greet"
Enter
Sleep 500ms
Type "Bob"
Enter
Sleep 1s
