Output gh.gif
Set Width 1200
Set Height 800
Set Theme "nord"
Set FontSize 18
Set Shell "zsh"
Env TERM "xterm-256color"
Env COLORTERM "truecolor"
Env PATH "/usr/bin:/bin:/usr/local/bin:$HOME/go/bin:$PATH"

# Build the example (unset NO_COLOR and clear while hidden)
Hide
Type "unset NO_COLOR && clear"
Enter
Sleep 100ms
Show
Type "go build -o gh ./cmd/gh"
Enter
Sleep 2s

# A private config directory for the aliases, and the example's plugins
# on PATH (hidden)
Hide
Type "export XDG_CONFIG_HOME=$(mktemp -d) PATH=$PWD/plugins:$PATH && clear"
Enter
Sleep 100ms
Show

# A table on a terminal
Type "./gh pr list"
Enter
Sleep 1.5s

# gh's --json, --jq and --template
Type "./gh pr list --json number,title,author.login"
Enter
Sleep 2s

Type `./gh pr list --jq '.[] | select(.state == "OPEN") | .title'`
Enter
Sleep 1.5s

Type `./gh pr list --template '{{range .}}{{tablerow (printf "#%v" .number) .title .author.login}}{{end}}'`
Enter
Sleep 2s

# Aliases, stored in the config file
Type "./gh alias set co 'pr checkout'"
Enter
Sleep 1s

Type "./gh co 321"
Enter
Sleep 1s

Type "./gh alias list"
Enter
Sleep 1.5s

# Plugins: plugins/gh-hello runs as "gh hello"
Type "./gh plugin list"
Enter
Sleep 1.5s

Type "./gh hello world"
Enter
Sleep 1.5s
