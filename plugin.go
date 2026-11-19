package clix

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
)

// Plugins are separate executables that add commands to an app, as
// kubectl's and gh's do: with App.Plugins set, "myapp foo args..." runs
// the program myapp-foo from PATH with args when foo is not a command.
//
// Security model:
//
//   - Running a plugin runs a program with the user's privileges, so a
//     plugin is trusted exactly as much as any other command on PATH:
//     whoever can write to a directory on PATH can add one. Nothing else
//     is searched.
//   - Only absolute directories on PATH are searched. Empty entries, "."
//     and other relative entries, which name the current directory, are
//     skipped, so a plugin is never found in the directory a command is run
//     from (unlike a shell with "." on PATH).
//   - The word must be a plugin name (a letter or digit followed by
//     letters, digits, hyphens and underscores), so it can never be a path
//     such as ../x.
//   - On Unix, a file that every user can write to (mode o+w) is not run.
//     On Windows only .exe and .com files are run, never .bat or .cmd
//     scripts, whose arguments cmd.exe would interpret.
//   - Plugins never shadow commands: they are looked up only for the first
//     argument, only when it names no command (built-in or added by an
//     extension, hidden or not) and the root takes no positional
//     arguments. A plugin named like a command never runs.
//   - The plugin gets the arguments after its name exactly as given, with
//     no shell in between, the app's standard input, output and error, and
//     the environment the app inherited plus the variables listed at
//     PluginEnv. No flag value, config value or secret of the app is
//     passed on.

// Plugin is an executable found on PATH that extends the app.
type Plugin struct {
	// Name is the command it adds: "foo" for myapp-foo.
	Name string

	// Path is the absolute path of the file that runs: the first one on
	// PATH.
	Path string

	// Shadowed lists the files with the same name later on PATH, which
	// never run.
	Shadowed []string

	// Command is the name of the command that has the plugin's name, which
	// always runs instead, or "".
	Command string

	// Insecure says why the file is not run (writable by every user), or
	// is "".
	Insecure string
}

// Runnable reports whether the plugin runs when its name is given: no
// command has its name and the file is not insecure.
func (p Plugin) Runnable() bool { return p.Command == "" && p.Insecure == "" }

// PluginExitError reports that a plugin exited with a status other than
// 0. ExitCode returns that status, so a program's exit status is the
// plugin's; Main exits with it without printing anything, since the plugin
// has reported its own error.
type PluginExitError struct {
	// Name and Path identify the plugin.
	Name, Path string

	// Code is the plugin's exit status: 128 plus the signal number when a
	// signal ended it, as shells report it.
	Code int
}

func (e *PluginExitError) Error() string {
	return fmt.Sprintf("plugin %s exited with status %d", filepath.Base(e.Path), e.Code)
}

// validPluginName reports whether word can name a plugin: a letter or
// digit followed by letters, digits, hyphens and underscores.
func validPluginName(word string) bool {
	if word == "" {
		return false
	}
	for i := 0; i < len(word); i++ {
		c := word[i]
		alnum := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
		if !alnum && (i == 0 || c != '-' && c != '_') {
			return false
		}
	}
	return true
}

// pluginPrefix is the file name prefix of the app's plugins: "myapp-".
func (a *App) pluginPrefix() string { return a.Name + "-" }

// pluginDirs returns the absolute directories on PATH, in order, without
// repeats.
func pluginDirs() []string {
	var dirs []string
	seen := map[string]bool{}
	for _, d := range filepath.SplitList(os.Getenv("PATH")) {
		if d == "" || !filepath.IsAbs(d) {
			continue // the current directory, or relative to it
		}
		d = filepath.Clean(d)
		if !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// pluginExtensions returns the file extensions a plugin may have: none on
// Unix, .exe and .com on Windows.
func pluginExtensions() []string {
	if runtime.GOOS == "windows" {
		return []string{".exe", ".com"}
	}
	return []string{""}
}

// checkPluginFile reports whether path is a file that may run as a plugin:
// a regular file (after following links) that is executable. insecure says
// why it must not run anyway.
func checkPluginFile(path string) (ok bool, insecure string) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false, ""
	}
	if runtime.GOOS == "windows" {
		return true, ""
	}
	if info.Mode().Perm()&0o111 == 0 {
		return false, ""
	}
	if info.Mode().Perm()&0o002 != 0 {
		return true, "it is writable by every user"
	}
	return true, ""
}

// LookupPlugin finds the plugin name on PATH (see Plugins), whether or not
// App.Plugins is set. It returns false when there is none or name is not a
// plugin name.
func (a *App) LookupPlugin(name string) (Plugin, bool) {
	if a.Name == "" || !validPluginName(name) {
		return Plugin{}, false
	}
	p := Plugin{Name: name}
	for _, dir := range pluginDirs() {
		for _, ext := range pluginExtensions() {
			path := filepath.Join(dir, a.pluginPrefix()+name+ext)
			ok, insecure := checkPluginFile(path)
			if !ok {
				continue
			}
			if p.Path == "" {
				p.Path, p.Insecure = path, insecure
			} else {
				p.Shadowed = append(p.Shadowed, path)
			}
		}
	}
	if p.Path == "" {
		return Plugin{}, false
	}
	if a.Root != nil {
		if cmd := a.Root.findChildMatch(name, a.CaseSensitive); cmd != nil {
			p.Command = cmd.Name
		}
	}
	return p, true
}

// ListPlugins returns the plugins on PATH, sorted by name, including the
// ones that do not run (see Plugin.Runnable), whether or not App.Plugins
// is set.
func (a *App) ListPlugins() []Plugin {
	if a.Name == "" {
		return nil
	}
	names := map[string]bool{}
	for _, dir := range pluginDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			rest, ok := strings.CutPrefix(e.Name(), a.pluginPrefix())
			if !ok {
				continue
			}
			for _, ext := range pluginExtensions() {
				if ext != "" {
					if !strings.EqualFold(filepath.Ext(rest), ext) {
						continue
					}
					rest = rest[:len(rest)-len(ext)]
				}
				if validPluginName(rest) {
					names[rest] = true
				}
				break
			}
		}
	}
	var out []Plugin
	for name := range names {
		if p, ok := a.LookupPlugin(name); ok {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// PluginEnv returns the variables set for the plugin name, on top of the
// environment the app inherited:
//
//	<PREFIX>_PLUGIN=1             the program runs as a plugin
//	<PREFIX>_PLUGIN_NAME=<name>   the command it was run as ("foo")
//	<PREFIX>_APP_NAME=<app>       App.Name
//	<PREFIX>_APP_VERSION=<v>      App.Version, when set
//	<PREFIX>_APP_PATH=<path>      the app's executable, to call it back
//
// where <PREFIX> is App.EnvPrefix. None of them holds a secret.
func (a *App) PluginEnv(name string) []string {
	env := []string{
		a.envVarName("plugin") + "=1",
		a.envVarName("plugin-name") + "=" + name,
		a.envVarName("app-name") + "=" + a.Name,
	}
	if a.Version != "" {
		env = append(env, a.envVarName("app-version")+"="+a.Version)
	}
	if exe, err := os.Executable(); err == nil {
		env = append(env, a.envVarName("app-path")+"="+exe)
	}
	return env
}

// pluginCandidate returns the first argument when Run may look it up as a
// plugin: a plugin name that names no root command, given first, while
// the root takes no positional arguments (so the word would be an error).
func (a *App) pluginCandidate(args []string) (string, bool) {
	if !a.Plugins || len(args) == 0 || a.Root == nil {
		return "", false
	}
	word := args[0]
	if !validPluginName(word) {
		return "", false
	}
	if a.Root.findChildMatch(word, a.CaseSensitive) != nil || a.skipsRootName(word) {
		return "", false
	}
	if a.Root.Run != nil && a.Root.Flags != nil && len(a.Root.Flags.PositionalFlags()) > 0 {
		return "", false
	}
	return word, true
}

// tryPlugin runs the plugin that args' first word names, if there is one
// (see Plugins and pluginCandidate), and reports whether it ran one.
func (a *App) tryPlugin(ctx context.Context, args []string) (bool, error) {
	word, ok := a.pluginCandidate(args)
	if !ok {
		return false, nil
	}
	p, found := a.LookupPlugin(word)
	if !found || p.Command != "" {
		return false, nil
	}
	return true, a.runPlugin(ctx, p, args[1:])
}

// runPlugin runs the plugin p with args, with the app's standard streams,
// and returns nil when it exits with status 0 and a *PluginExitError
// otherwise.
//
// The plugin runs in the foreground: the terminal's Ctrl-C reaches it
// directly, since it is in the same process group, so while it runs the
// app ignores SIGINT itself (it keeps waiting for the plugin) and forwards
// SIGTERM to it. Cancelling ctx does not stop the plugin, because Main
// cancels ctx on the Ctrl-C the plugin is already handling.
func (a *App) runPlugin(_ context.Context, p Plugin, args []string) error {
	if p.Insecure != "" {
		return fmt.Errorf("not running plugin %s: %s (fix it with: chmod o-w %s)", p.Path, p.Insecure, p.Path)
	}
	cmd := exec.Command(p.Path, args...)
	var in io.Reader = os.Stdin
	if a.In != nil {
		in = a.In
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = in, a.stdout(), a.stderr()
	cmd.Env = append(os.Environ(), a.PluginEnv(p.Name)...)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("plugin %s: %w", p.Path, err)
	}
	a.ranPlugin = true
	done := make(chan struct{})
	go func() {
		for {
			select {
			case sig := <-signals:
				if sig == syscall.SIGTERM {
					if err := cmd.Process.Signal(sig); err != nil {
						_ = cmd.Process.Kill()
					}
				}
			case <-done:
				return
			}
		}
	}()
	err := cmd.Wait()
	close(done)
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code := exit.ExitCode()
		if ws, ok := exit.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			code = 128 + int(ws.Signal())
		}
		return &PluginExitError{Name: p.Name, Path: p.Path, Code: code}
	}
	if err != nil {
		return fmt.Errorf("plugin %s: %w", p.Path, err)
	}
	return nil
}
