Output basic-usage.gif
Width 1200
Height 800
Theme "Monokai Pro"
FontSize 18
Shell "bash"

# Set working directory to project root
Set CWD "../../.."

# Show basic command structure
Type "cd examples/basic"
Enter
Sleep 500ms

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

