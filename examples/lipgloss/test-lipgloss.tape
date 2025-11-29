Output test-lipgloss.gif
Set Width 1200
Set Height 800
Set Theme "Monokai Pro"
Set FontSize 18
Set Shell "bash"

Type "export TERM=xterm-256color && export COLORTERM=truecolor && go run ./cmd/styled style --format json 2>&1 | head -20"
Enter
Sleep 3s
