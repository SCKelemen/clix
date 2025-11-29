Output gh-example.gif
Width 1200
Height 800
Theme "Monokai Pro"
FontSize 18
Shell "bash"

Set CWD "../../.."

# Navigate to gh example
Type "cd examples/gh"
Enter
Sleep 500ms

# Show help
Type "./gh"
Enter
Sleep 1s

# Show auth help
Type "./gh auth --help"
Enter
Sleep 1.5s

# Show repo commands
Type "./gh repo --help"
Enter
Sleep 1.5s

# Show pr commands
Type "./gh pr --help"
Enter
Sleep 1.5s

# Show org commands
Type "./gh org --help"
Enter
Sleep 1.5s

# Show version
Type "./gh --version"
Enter
Sleep 1s

