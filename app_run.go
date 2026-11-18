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

	// Load configuration up front, but report a broken config file only after
	// --help and --version have had a chance to run.
	configErr := a.ensureConfigLoaded(ctx)

	// Use Flags() to get root command's flags (symmetric with cmd.Flags).
	// It also guarantees the root flag set exists and is non-strict.
	rootFlags := a.Flags()

	// Find the command first, then parse every flag exactly once: flags bind
	// to the innermost command that defines them and fall back to the root.
	cmd, positionals, stray, err := a.parseCommandLine(args)
	if err != nil {
		return err
	}

	// Check if global --version flag was set
	if version, _ := rootFlags.Bool("version"); version {
		// Show version info - same format as "version" command but simpler (no commit/date)
		// The version extension sets app.Version, so if it's enabled, we show it
		if a.Version != "" {
			fmt.Fprintf(a.Out, "%s version %s\n", a.Name, a.Version)
		} else {
			fmt.Fprintf(a.Out, "%s\n", a.Name)
		}
		return nil
	}

	// --help/-h (automatic for all commands) takes precedence over
	// everything else, including positional arguments and required flags.
	if helpRequested(cmd) {
		return a.printCommandHelp(cmd)
	}

	// A pure group (children but no Run handler) followed by a word that is
	// not one of its children is a mistyped command.
	if stray != "" && len(cmd.Children) > 0 && cmd.Run == nil {
		return fmt.Errorf("unknown command: %s %s", cmd.Path(), stray)
	}

	// Map leftover positional args to flags marked Positional: true
	if len(positionals) > 0 {
		excess, err := cmd.Flags.MapPositionals(positionals)
		if err != nil {
			return err
		}
		if len(excess) > 0 {
			return fmt.Errorf("unexpected arguments: %s", strings.Join(excess, " "))
		}
	}

	// Count user-defined children (groups or commands, excluding default commands like help, config, autocomplete)
	userChildren := a.countUserChildren(cmd)

	// If command has user-defined children:
	// - If it has a Run handler, execute it (command with children can have default behavior)
	// - If it has no Run handler, show help (group behavior)
	if userChildren > 0 && cmd.Run == nil {
		return a.printCommandHelp(cmd)
	}

	if configErr != nil {
		return configErr
	}

	// Fill in flags the command line did not set, in precedence order:
	// environment, then config file, then defaults.
	if err := a.applyConfigToFlags(rootFlags); err != nil {
		return err
	}
	if cmd.Flags != rootFlags {
		if err := a.applyConfigToFlags(cmd.Flags); err != nil {
			return err
		}
	}

	// Three-way mode detection for required flags:
	// 1. No CLI flags passed + required missing → interactive prompting
	// 2. All required satisfied (from any source) → run
	// 3. Some CLI flags passed + required missing → error
	// Root (app-level) flags count as "flags passed" too: "mk --project p"
	// is a scripted invocation even though --project lives on the root.
	missing := cmd.Flags.MissingRequired()
	if len(missing) > 0 {
		if cmd.Flags.AnyCLISet() || rootFlags.AnyCLISet() {
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

// parseCommandLine resolves the command path in args and parses all flags
// in a single pass.
//
// The path is found first: words that name a child command descend into it,
// and flag tokens are skipped (together with their value, when the flag is
// known to take one). Parsing then binds each flag to the command in effect
// where it appears, falling back to the root flag set, so a command's -f is
// never shadowed by a root -f. Flags after the command name therefore resolve
// against the command before the root. A bare "--" ends flag parsing and
// everything after it is positional.
//
// It returns the matched command, the positional arguments, and the first
// word that did not match a child command (stray), if any.
func (a *App) parseCommandLine(args []string) (cmd *Command, positionals []string, stray string, err error) {
	root := a.Root
	cmd = root
	path := []*Command{root}
	scope := make([]*Command, len(args)) // command in effect for each flag token
	isPath := make([]bool, len(args))    // tokens that name the command path
	sawWord := false

	// Pass 1: find the command path.
scan:
	for i := 0; i < len(args); i++ {
		tok := args[i]
		switch {
		case tok == "--":
			break scan
		case isFlagArg(tok):
			scope[i] = cmd
			name, _, hasValue := strings.Cut(tok, "=")
			if f := lookupArg(name, cmd.Flags, root.Flags); f != nil && !hasValue && takesValue(f) {
				i++ // skip the flag's value
			}
		default:
			if !sawWord {
				sawWord = true
				// The binary may be invoked as "app-name app-name child".
				if strings.EqualFold(tok, root.Name) {
					isPath[i] = true
					continue
				}
			}
			child := cmd.findChild(tok)
			if child == nil {
				stray = tok
				break scan
			}
			isPath[i] = true
			cmd = child
			path = append(path, child)
		}
	}

	for _, c := range path {
		resetFlagState(c.Flags)
	}

	// Pass 2: parse flags and collect positionals.
	for i := 0; i < len(args); i++ {
		if isPath[i] {
			continue
		}
		tok := args[i]
		if tok == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !isFlagArg(tok) {
			positionals = append(positionals, tok)
			continue
		}

		sc := scope[i]
		if sc == nil {
			sc = cmd
		}
		name, value, hasValue := strings.Cut(tok, "=")
		f := lookupArg(name, sc.Flags, root.Flags)
		if f == nil && sc != cmd {
			// A switch written before the command name, such as
			// "app --force delete", can still belong to the command. Only
			// accept it when it does not need a separate value, which pass 1
			// could not have known to skip.
			if g := lookupArg(name, cmd.Flags); g != nil && (hasValue || !takesValue(g)) {
				f = g
			}
		}
		if f == nil {
			if cmd.Flags.strict {
				return nil, nil, "", fmt.Errorf("unknown flag: %s", name)
			}
			positionals = append(positionals, tok)
			continue
		}
		err := setFromArg(f, name, value, hasValue, func() (string, bool) {
			if i+1 >= len(args) {
				return "", false
			}
			i++
			return args[i], true
		})
		if err != nil {
			return nil, nil, "", err
		}
	}

	return cmd, positionals, stray, nil
}

// helpRequested reports whether --help was passed for cmd or any command on
// its path, including the root.
func helpRequested(cmd *Command) bool {
	for c := cmd; c != nil; c = c.parent {
		if c.Flags == nil {
			continue
		}
		if help, _ := c.Flags.Bool("help"); help {
			return true
		}
	}
	return false
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

	path, err := a.configFilePath()
	if err != nil {
		// Without a home directory there is no config file to read; that is
		// not an error for commands that do not save configuration.
		return nil
	}
	if a.Config == nil {
		a.Config = NewConfigManager(a.Name)
	}
	a.configLoadErr = a.Config.Load(path)
	return a.configLoadErr
}

// applyConfigToFlags fills in every flag the command line did not set,
// trying environment variables, then the config file, then the flag's
// default. It must run after parsing so command-line values keep the highest
// precedence. A value that the flag rejects is reported as an error naming
// its source.
func (a *App) applyConfigToFlags(flags *FlagSet) error {
	if flags == nil {
		return nil
	}

	for _, flag := range flags.flags {
		if flag.cliSet {
			continue
		}

		// The built-in --help and --version switches only come from the
		// command line; MYAPP_HELP or MYAPP_VERSION must not hijack every run.
		if isCommandLineOnly(flag) {
			continue
		}

		// Try each source in order of precedence
		if ok, err := a.trySetFromEnv(flag); ok || err != nil {
			if err != nil {
				return err
			}
			continue
		}
		if ok, err := a.trySetFromConfig(flag); ok || err != nil {
			if err != nil {
				return err
			}
			continue
		}
		a.trySetFromDefault(flag)
	}
	return nil
}

// isCommandLineOnly reports whether flag is a boolean --help or --version
// switch, which are never read from the environment or the config file.
func isCommandLineOnly(flag *Flag) bool {
	if flag.Name != "help" && flag.Name != "version" {
		return false
	}
	_, ok := flag.Value.(boolFlag)
	return ok
}

// envVarName returns the default environment variable for a flag or key:
// the app's EnvPrefix, an underscore, and the name upper-cased with hyphens
// replaced by underscores (for example, MYAPP_DRY_RUN).
func (a *App) envVarName(name string) string {
	return fmt.Sprintf("%s_%s", a.EnvPrefix, strings.ToUpper(strings.ReplaceAll(name, "-", "_")))
}

// trySetFromEnv attempts to set a flag value from environment variables.
// It reports whether a variable was found; a value the flag rejects is
// returned as an error that names the variable. Empty variables are treated
// as unset.
func (a *App) trySetFromEnv(flag *Flag) (bool, error) {
	name, val, ok := a.lookupFlagEnv(flag)
	if !ok {
		return false, nil
	}
	if err := setAndValidate(flag, val); err != nil {
		return false, fmt.Errorf("invalid value %q for --%s from environment variable %s: %w", val, flag.Name, name, err)
	}
	flag.set = true
	flag.source = SourceEnvVar
	return true, nil
}

// setAndValidate parses raw into flag and then runs the flag's Validate
// function, mirroring what the command-line parser does.
func setAndValidate(flag *Flag, raw string) error {
	if err := flag.Value.Set(raw); err != nil {
		return err
	}
	if flag.Validate != nil {
		return flag.Validate(raw)
	}
	return nil
}

// flagEnvVars lists the environment variables consulted for flag, in order:
// EnvVar, then each of EnvVars, then the APP_KEY default.
func (a *App) flagEnvVars(flag *Flag) []string {
	names := make([]string, 0, len(flag.EnvVars)+2)
	seen := make(map[string]bool, cap(names))
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	add(flag.EnvVar)
	for _, name := range flag.EnvVars {
		add(name)
	}
	add(a.envVarName(flag.Name))
	return names
}

// lookupFlagEnv returns the first non-empty environment variable among
// flagEnvVars(flag), with its name.
func (a *App) lookupFlagEnv(flag *Flag) (name, value string, ok bool) {
	for _, name := range a.flagEnvVars(flag) {
		if val, found := os.LookupEnv(name); found && val != "" {
			return name, val, true
		}
	}
	return "", "", false
}

// trySetFromConfig attempts to set a flag value from configuration.
// It reports whether a value was found; a value the flag rejects is returned
// as an error that names the config key.
func (a *App) trySetFromConfig(flag *Flag) (bool, error) {
	if a.Config == nil {
		return false, nil
	}

	val, ok := a.Config.Get(flag.Name)
	if !ok {
		return false, nil
	}
	if err := setAndValidate(flag, val); err != nil {
		return false, fmt.Errorf("invalid value %q for --%s from config file key %q: %w", val, flag.Name, flag.Name, err)
	}
	flag.set = true
	flag.source = SourceConfigFile
	return true, nil
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
		flag.source = SourcePrompt
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
