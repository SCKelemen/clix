Output test-color-check.gif
Set Width 1200
Set Height 800
Set Theme "Monokai Pro"
Set FontSize 18
Set Shell "bash"
Env TERM "xterm-256color"
Env COLORTERM "truecolor"

Type "unset NO_COLOR && tput colors && echo -e '\033[31mRED\033[0m \033[32mGREEN\033[0m \033[34mBLUE\033[0m'"
Enter
Sleep 1s
