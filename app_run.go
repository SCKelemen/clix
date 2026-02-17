package clix

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Run executes the application with the given context and arguments.
// If args is nil, os.Args[1:] is used.
// The context is propagated to command handlers and can be used for cancellation.
func (a *App) Run(ctx context.Context, args []string) error {
	if a.Root == nil {
		return errors.New("clix: no root command configured")
	}

	a.ensureRootPrepared()

	// Apply extensions (extensions add optional commands)
	if err := a.ApplyExtensions(); err != nil {
		return err
	}

	if args == nil {
		args = os.Args[1:]
	}

	if err := a.ensureConfigLoaded(ctx); err != nil {
		return err
	}

	// Use Flags() to get root command's flags (symmetric with cmd.Flags)
	flags := a.Flags()
	// Apply config/env/defaults to root flags before parsing
	// This sets defaults, but parsing will override if flags are provided
	a.applyConfigToFlags(a.Root.Flags, true)
	remaining, err := flags.Parse(args)
	if err != nil {
		return err
	}

	// Check if global --version flag was set
	if version, _ := flags.Bool("version"); version {
		// Show version info - same format as "version" command but simpler (no commit/date)
		// The version extension sets app.Version, so if it's enabled, we show it
		if a.Version != "" {
			fmt.Fprintf(a.Out, "%s version %s\n", a.Name, a.Version)
		} else {
			fmt.Fprintf(a.Out, "%s\n", a.Name)
		}
		return nil
	}

	// Check if global --help flag was set (when --help appears before any command)
	if help, _ := flags.Bool("help"); help {
		// If there are remaining args, they might be a command - match it first
		// so we show help for that command instead of root
		if len(remaining) > 0 {
			if cmd, _ := a.matchCommand(remaining); cmd != nil {
				return a.printCommandHelp(cmd)
			}
		}
		return a.printCommandHelp(a.Root)
	}

	cmd, rest := a.matchCommand(remaining)
	if cmd == nil {
		if len(remaining) == 0 {
			return a.printCommandHelp(a.Root)
		}
		// Unknown command - show help for parent or error
		// Try to find the parent command to show its help
		if len(remaining) > 1 {
			// Try to match parent command
			if parentCmd, _ := a.matchCommand(remaining[:len(remaining)-1]); parentCmd != nil {
				return a.printCommandHelp(parentCmd)
			}
		}
		return fmt.Errorf("unknown command: %s", strings.Join(remaining, " "))
	}

	// Check if we tried to match a child but it doesn't exist
	// (i.e., we have remaining args that look like a command name but didn't match)
	// Only show error if the command has no Run handler (it's a pure group)
	if len(rest) > 0 && len(cmd.Children) > 0 && cmd.Run == nil {
		// Check if the first remaining arg looks like it could be a child command
		// (not a flag and not already matched)
		firstArg := rest[0]
		if !strings.HasPrefix(firstArg, "-") {
			// This looks like a command name but didn't match - show error
			parentPath := cmd.Path()
			return fmt.Errorf("unknown command: %s %s", parentPath, firstArg)
		}
	}
	// If the command has a Run handler, we'll let it handle the args (even if they don't match a child)

	// Ensure defaults and env/config values are applied prior to parsing.
	// This sets defaults, but parsing will override if flags are provided
	// Use reset=false to avoid resetting flags that were already set on root
	a.applyConfigToFlags(cmd.Flags, false)

	// Parse flags first - flags consume arguments starting with -
	// This handles: --flag=value, --flag value, -f=value, -f value
	resultArgs, err := cmd.Flags.Parse(rest)
	if err != nil {
		return err
	}

	// Map leftover positional args to flags marked Positional: true
	if len(resultArgs) > 0 {
		excess, err := cmd.Flags.MapPositionals(resultArgs)
		if err != nil {
			return err
		}
		if len(excess) > 0 {
			return fmt.Errorf("unexpected arguments: %s", strings.Join(excess, " "))
		}
	}

	// Check for --help/-h flag at command level (automatic for all commands)
	// Help flags are automatically added to every command in NewCommand/prepare
	// This takes precedence over everything else - no need to implement per command
	if help, _ := cmd.Flags.Bool("help"); help {
		return a.printCommandHelp(cmd)
	}

	// Count user-defined children (groups or commands, excluding default commands like help, config, autocomplete)
	userChildren := a.countUserChildren(cmd)

	// If command has user-defined children:
	// - If it has a Run handler, execute it (command with children can have default behavior)
	// - If it has no Run handler, show help (group behavior)
	if userChildren > 0 && cmd.Run == nil {
		return a.printCommandHelp(cmd)
	}

	// Three-way mode detection for required flags:
	// 1. No CLI flags passed + required missing → interactive prompting
	// 2. All required satisfied (from any source) → run
	// 3. Some CLI flags passed + required missing → error
	missing := cmd.Flags.MissingRequired()
	if len(missing) > 0 {
		if cmd.Flags.AnyCLISet() {
			// Mode 3: some flags provided, required missing → error
			names := make([]string, len(missing))
			for i, f := range missing {
				names[i] = "--" + f.Name
			}
			return fmt.Errorf("missing required flags: %s", strings.Join(names, ", "))
		}
		// Mode 1: no CLI flags → interactive prompting
		if err := a.promptForRequiredFlags(ctx, cmd, missing); err != nil {
			return err
		}
	}

	// Create context for handler execution
	runCtx := &Context{
		Context: ctx,
		App:     a,
		Command: cmd,
	}

	if cmd.PreRun != nil {
		if err := cmd.PreRun(runCtx); err != nil {
			return err
		}
	}

	if cmd.Run == nil {
		return fmt.Errorf("command %s has no run handler (did you intend this to be a group?)", cmd.Path())
	}

	if err := cmd.Run(runCtx); err != nil {
		return err
	}

	if cmd.PostRun != nil {
		if err := cmd.PostRun(runCtx); err != nil {
			return err
		}
	}

	return nil
}

// matchCommand matches commands starting from the root, handling the case where
// the root command name appears in the arguments.
func (a *App) matchCommand(args []string) (*Command, []string) {
	// If the first argument matches the root command name, skip it
	// (this happens when the binary is invoked as "app-name root-command child")
	if len(args) > 0 && strings.EqualFold(args[0], a.Root.Name) {
		return a.Root.match(args[1:])
	}
	return a.Root.match(args)
}

func (a *App) ensureRootPrepared() {
	if a.Root == nil || a.rootPrepared {
		return
	}
	a.Root.prepare(nil)
	a.rootPrepared = true
}

func (a *App) ensureConfigLoaded(ctx context.Context) error {
	if a.configLoaded {
		return a.configLoadErr
	}
	a.configLoaded = true

	path, err := a.ConfigFile()
	if err != nil {
		a.configLoadErr = err
		return err
	}
	a.configLoadErr = a.Config.Load(path)
	return a.configLoadErr
}

// applyConfigToFlags applies env vars, config, and defaults to flags.
// This should be called BEFORE parsing. After parsing, flags that were set
// will have flag.set = true and won't be overridden.
// If reset is false, flags that are already set (flag.set == true) will be skipped.
func (a *App) applyConfigToFlags(flags *FlagSet, reset bool) {
	if flags == nil {
		return
	}

	for _, flag := range flags.flags {
		// If flag was already set (e.g., by parsing) and we're not resetting, skip it
		// This ensures flags > env > config > defaults precedence
		if !reset && flag.set {
			continue
		}

		// Reset flag state before applying precedence
		flag.set = false

		// Try each source in order of precedence
		if a.trySetFromEnv(flag) {
			continue
		}
		if a.trySetFromConfig(flag) {
			continue
		}
		a.trySetFromDefault(flag)
	}
}

// trySetFromEnv attempts to set a flag value from environment variables.
// Returns true if a value was found and set.
func (a *App) trySetFromEnv(flag *Flag) bool {
	// Try explicit EnvVar first
	if flag.EnvVar != "" {
		if val, ok := os.LookupEnv(flag.EnvVar); ok {
			flag.Value.Set(val)
			flag.set = true
			return true
		}
	}

	// Try default pattern (APP_KEY)
	upper := fmt.Sprintf("%s_%s", a.EnvPrefix, strings.ToUpper(strings.ReplaceAll(flag.Name, "-", "_")))
	if val, ok := os.LookupEnv(upper); ok {
		flag.Value.Set(val)
		flag.set = true
		return true
	}

	return false
}

// trySetFromConfig attempts to set a flag value from configuration.
// Returns true if a value was found and set.
func (a *App) trySetFromConfig(flag *Flag) bool {
	if a.Config == nil {
		return false
	}

	if val, ok := a.Config.Values()[flag.Name]; ok {
		flag.Value.Set(val)
		flag.set = true
		return true
	}

	return false
}

// trySetFromDefault sets a flag value from its default if available.
func (a *App) trySetFromDefault(flag *Flag) {
	if flag.Default != "" {
		flag.Value.Set(flag.Default)
	}
}

// promptForRequiredFlags interactively prompts for each missing required flag.
func (a *App) promptForRequiredFlags(ctx context.Context, cmd *Command, missing []*Flag) error {
	for _, flag := range missing {
		label := flag.Prompt
		if label == "" {
			label = strings.ReplaceAll(flag.Name, "-", " ")
			if len(label) > 0 {
				label = strings.ToUpper(label[:1]) + label[1:]
			}
		}

		value, err := a.Prompter.Prompt(ctx, PromptRequest{
			Label: label,
			Theme: a.DefaultTheme,
		})
		if err != nil {
			return err
		}
		if err := flag.Value.Set(value); err != nil {
			return err
		}
		if flag.Validate != nil {
			if err := flag.Validate(value); err != nil {
				return err
			}
		}
		flag.set = true
	}
	return nil
}

func (a *App) printCommandHelp(cmd *Command) error {
	helper := HelpRenderer{App: a, Command: cmd}
	return helper.Render(a.Out)
}

// countUserChildren returns the count of child commands/groups that are not extension commands.
func (a *App) countUserChildren(cmd *Command) int {
	if cmd == nil || len(cmd.Children) == 0 {
		return 0
	}

	count := 0
	for _, child := range cmd.Children {
		if !child.IsExtensionCommand {
			count++
		}
	}
	return count
}
