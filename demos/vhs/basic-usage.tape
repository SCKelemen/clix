Output basic-usage.gif
Set Width 1200
Set Height 800
Set Theme "Monokai Pro"
Set FontSize 18
Set Shell "bash"

# Build and show basic command structure
Type "cd examples/basic"
Enter
Sleep 500ms

Type "go build -o demo ./cmd/demo"
Enter
Sleep 2s

Type "./demo"
Enter
Sleep 1s

# Show help
Type "./demo --help"
Enter
Sleep 1.5s

# Show greet command help
Type "./demo greet --help"
Enter
Sleep 1.5s

# Run greet with argument
Type "./demo greet Alice"
Enter
Sleep 1s

# Run greet without argument (will prompt)
Type "./demo greet"
Enter
Sleep 500ms
Type "Bob"
Enter
Sleep 1s

# Show root command
Type "./demo"
Enter
Sleep 1s

