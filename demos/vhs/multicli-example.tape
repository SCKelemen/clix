Output multicli-example.gif
Width 1200
Height 800
Theme "Monokai Pro"
FontSize 18
Shell "bash"

Set CWD "../../.."

# Navigate to multicli example
Type "cd examples/multicli"
Enter
Sleep 500ms

# Show dev CLI
Type "./dev --help"
Enter
Sleep 1.5s

Type "./dev database list"
Enter
Sleep 1s

# Show db CLI (focused)
Type "./db --help"
Enter
Sleep 1.5s

Type "./db list"
Enter
Sleep 1s

# Show sec CLI with aliases
Type "./sec --help"
Enter
Sleep 1.5s

Type "./sec vulns list"
Enter
Sleep 1s

# Show bq CLI with versioning
Type "./bq --help"
Enter
Sleep 1.5s

Type "./bq dataset list"
Enter
Sleep 1s

