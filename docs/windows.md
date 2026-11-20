# Windows notes

clix runs on Windows with the same API and the same rules as on Unix. This page collects what differs. CI runs the tests on Linux, and the shell completion tests on Linux and macOS, but nothing on Windows; contributors check that the Windows code builds with `GOOS=windows go vet ./...` (see [CONTRIBUTING.md](../CONTRIBUTING.md)).

## Terminal and color

- **Raw-mode prompts** (with the prompt extension) need a console with virtual terminal support. On Windows, raw mode turns on virtual terminal processing and input for the console first and restores the previous modes afterwards; a legacy console without virtual terminal support uses line mode. See [Raw mode and line mode](prompting-and-terminals.md#raw-mode-and-line-mode).
- **Color** in help, prompts, tables and progress indicators also needs virtual terminal processing. clix turns it on for the console and leaves it on, as other console programs do. Legacy consoles (before Windows 10 1511) get plain text. `NO_COLOR`, `CLICOLOR` and `CLICOLOR_FORCE` work as on other systems.
- **Symbols**: outside UTF-8 locales (decided by `LC_ALL`, `LC_CTYPE` and `LANG`), symbols fall back to ASCII. On Unix, a process with none of the three set is in the "C" locale and gets ASCII. On Windows, whose consoles take UTF-16 and render Unicode, it gets the Unicode symbols (`✓`, `…`, the braille spinner).
- **Resizing**: Windows has no signal for a resized console, so an open select prompt picks up the new size the next time it redraws.

## Keys and secrets

- **Key input**: on Windows, and for inputs that cannot be polled (descriptors above `FD_SETSIZE` on macOS, readers that are not files), keys are read in a goroutine. A byte it reads after a prompt ended (for example after Esc) goes to the next prompt or `clix.ReadLine` on the same input, in order; code that reads that input directly, not through clix, can miss it.
- **Secrets** are read with echo off through the console mode on Windows, as through the terminal settings on Linux, macOS and the BSDs. On other systems a secret is read with echo on.

## Signals

`App.Main` catches `os.Interrupt` (Ctrl-C) and `syscall.SIGTERM`, as on Unix (see the [Quick Start](../README.md#quick-start)). A running progress indicator restores the console when its context ends. Without `Main`, on Unix, a running spinner also catches SIGINT and SIGTERM itself; on Windows it relies on the context, so cancel the context on Ctrl-C (for example with `signal.NotifyContext`) or use `Main`. See [Progress for long waits](prompting-and-terminals.md#progress-for-long-waits).

## Files

- **The config file** is `config.yaml` in `%XDG_CONFIG_HOME%\<app>` when `XDG_CONFIG_HOME` is set, and otherwise in `.config\<app>` in the home directory (`%USERPROFILE%`), as on Unix; clix does not use `%AppData%`. `App.ConfigPath` overrides it. See [Configuration](configuration.md#the-config-file).
- **Permissions**: clix writes the config file with mode 0600 in a 0700 directory, but Windows ignores Unix permission bits other than the owner's write bit (Go maps it to the read-only attribute). The file is protected by the access control list it inherits from its directory, which for the default location is your user profile.

## Plugins

With the [plugin extension](extensions.md#plugin-clixextplugin), only `.exe` and `.com` files run as plugins on Windows, never `.bat` or `.cmd` scripts, whose arguments `cmd.exe` would interpret. The Unix checks that a plugin is executable and not writable by every user do not apply: any regular file named `<app>-<name>.exe` or `<app>-<name>.com` in an absolute PATH directory runs.

## Shell completion

`myapp autocomplete powershell` prints a PowerShell completion script: save the output to a file and dot-source it from `$PROFILE`. See [Shell completion](completion.md).
