package clix

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// Run executes the application with the given context and arguments.
// If args is nil, os.Args[1:] is used.
// The context is propagated to command handlers and can be used for cancellation.
//
// Run is not safe for concurrent use on the same App: it resets and fills
// the flags of the command tree. Calling Run several times in sequence is
// supported: each Run first restores every flag (and its destination
// variable) to the value it held before the first Run, so values parsed by
// one Run never leak into the next. To run commands in parallel, build one
// App per goroutine.
func (a *App) Run(ctx context.Context, args []string) error {
	if a.Root == nil {
		return errors.New("clix: no root command configured")
	}

	// Adopt the root (it may have been replaced since NewApp).
	a.bindRoot()

	// Prepare the tree on every Run: commands appended to Children (or a
	// new Root) since the last Run need a flag set, --help and parent links.
	// Preparing is idempotent.
	a.prepareTree()

	// Apply extensions (extensions add optional commands), then prepare
	// whatever they attached directly to Children.
	if err := a.ApplyExtensions(); err != nil {
		return err
	}
	a.prepareTree()

	if args == nil {
		args = os.Args[1:]
	}

	// Load configuration up front, but report a broken config file only after
	// --help and --version have had a chance to run.
	configErr := a.ensureConfigLoaded(ctx)

	rootFlags := a.Root.Flags

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
			fmt.Fprintf(a.stdout(), "%s version %s\n", a.Name, a.Version)
		} else {
			fmt.Fprintf(a.stdout(), "%s\n", a.Name)
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
		return usageErrorf("unknown command: %s %s", cmd.Path(), stray)
	}

	// Map leftover positional args to flags marked Positional: true
	if len(positionals) > 0 {
		excess, err := cmd.Flags.MapPositionals(positionals)
		if err != nil {
			return err
		}
		if len(excess) > 0 {
			return usageErrorf("unexpected arguments: %s", strings.Join(excess, " "))
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
	// environment, then config file, then defaults. Every command on the
	// path takes part: the root, each group, and the command itself.
	path := commandPath(cmd)
	for _, c := range path {
		if err := a.applyConfigToFlags(c.Flags); err != nil {
			return err
		}
	}

	// Three-way mode detection for required flags:
	// 1. No CLI flags passed + required missing → interactive prompting
	//    (only when the app is interactive; otherwise an error)
	// 2. All required satisfied (from any source) → run
	// 3. Some CLI flags passed + required missing → error
	// Flags anywhere on the path count as "flags passed": "mk --project p"
	// is a scripted invocation even though --project lives on the root.
	missing := missingRequired(cmd)
	if len(missing) > 0 {
		anyCLI := false
		for _, c := range path {
			anyCLI = anyCLI || c.Flags.AnyCLISet()
		}
		if anyCLI || !a.IsInteractive() {
			// Mode 3 (or nobody to ask): report what is missing.
			return a.missingFlagsError(missing)
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
				if a.skipsRootName(tok) {
					isPath[i] = true
					continue
				}
			}
			child := cmd.findChildMatch(tok, a.CaseSensitive)
			if child == nil {
				stray = tok
				break scan
			}
			isPath[i] = true
			cmd = child
		}
	}

	// Start every Run from the values flags held before the first one, so
	// nothing parsed by a previous Run (on any command) carries over.
	resetTree(root)

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
				return nil, nil, "", usageErrorf("unknown flag: %s", name)
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

// skipsRootName reports whether word, the first word on the command line,
// is a repeat of the root's name ("app app child") that should be
// skipped. It is skipped only when it cannot mean anything else: no child
// has that name, and the root either has no Run handler or takes no
// positional arguments. Otherwise "echo echo" would lose its argument and a
// child named like the app would be unreachable.
func (a *App) skipsRootName(word string) bool {
	root := a.Root
	if !a.namesMatch(word, root.Name) {
		return false
	}
	if root.findChildMatch(word, a.CaseSensitive) != nil {
		return false
	}
	return root.Run == nil || len(root.Flags.PositionalFlags()) == 0
}

// namesMatch reports whether word names a command called name, using the
// same rules as command routing (see CaseSensitive).
func (a *App) namesMatch(word, name string) bool {
	if a.CaseSensitive {
		return word == name
	}
	return equalFoldASCII(word, name)
}

// commandPath returns the commands from the root down to cmd.
func commandPath(cmd *Command) []*Command {
	var path []*Command
	for c := cmd; c != nil; c = c.parent {
		path = append(path, c)
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// missingRequired returns the required flags without a value on cmd's
// path, outermost command first. A command added by an extension
// (IsExtensionCommand), and everything below it, only checks the flags
// from that command down, so "app version" never asks for a required
// --project on the root.
func missingRequired(cmd *Command) []*Flag {
	path := commandPath(cmd)
	from := 0
	for i := len(path) - 1; i >= 0; i-- {
		if path[i].IsExtensionCommand {
			from = i
			break
		}
	}
	var missing []*Flag
	for _, c := range path[from:] {
		missing = append(missing, c.Flags.MissingRequired()...)
	}
	return missing
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

// prepareTree links every command to its parent and gives each one a flag
// set and a --help flag. It is idempotent, so Run calls it every time.
func (a *App) prepareTree() {
	if a.Root == nil {
		return
	}
	a.Root.prepare(nil)
}

// stdout returns Out, or os.Stdout when Out is nil (for example, in a
// zero-value App).
func (a *App) stdout() io.Writer {
	if a.Out == nil {
		return os.Stdout
	}
	return a.Out
}

// stderr returns Err, or os.Stderr when Err is nil.
func (a *App) stderr() io.Writer {
	if a.Err == nil {
		return os.Stderr
	}
	return a.Err
}

// prompter returns Prompter, or a TextPrompter on In and Err when it is
// nil.
func (a *App) prompter() Prompter {
	if a.Prompter != nil {
		return a.Prompter
	}
	var in io.Reader = os.Stdin
	if a.In != nil {
		in = a.In
	}
	return TextPrompter{In: in, Out: a.stderr()}
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

		// CommandLineOnly flags, and the built-in --help and --version
		// switches, only come from the command line; MYAPP_FORCE or
		// MYAPP_HELP must not hijack every run.
		if isCommandLineOnly(flag) {
			a.trySetFromDefault(flag)
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

// isCommandLineOnly reports whether flag is never read from the
// environment or the config file: it is marked CommandLineOnly, or it is a
// boolean --help or --version switch.
func isCommandLineOnly(flag *Flag) bool {
	return flag.CommandLineOnly || isBuiltinSwitch(flag)
}

// isBuiltinSwitch reports whether flag is a boolean --help or --version
// switch.
func isBuiltinSwitch(flag *Flag) bool {
	if flag.Name != "help" && flag.Name != "version" {
		return false
	}
	_, ok := flag.Value.(boolFlag)
	return ok
}

// autoEnv reports whether flags read <EnvPrefix>_<NAME> (App.AutoEnv).
func (a *App) autoEnv() bool {
	return a.AutoEnv == nil || *a.AutoEnv
}

// autoConfig reports whether flags read the config key named after them
// (App.AutoConfig).
func (a *App) autoConfig() bool {
	return a.AutoConfig == nil || *a.AutoConfig
}

// flagConfigKey returns the config file key that supplies flag, if any.
func (a *App) flagConfigKey(flag *Flag) (string, bool) {
	switch {
	case isCommandLineOnly(flag) || flag.NoConfig:
		return "", false
	case flag.ConfigKey != "":
		return flag.ConfigKey, true
	case a.autoConfig():
		return flag.Name, true
	}
	return "", false
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
// EnvVar, then each of EnvVars, then the APP_KEY default (unless AutoEnv is
// off). Flags that are CommandLineOnly or NoEnv have none.
func (a *App) flagEnvVars(flag *Flag) []string {
	if isCommandLineOnly(flag) || flag.NoEnv {
		return nil
	}
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
	if a.autoEnv() {
		add(a.envVarName(flag.Name))
	}
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
	key, ok := a.flagConfigKey(flag)
	if !ok {
		return false, nil
	}

	val, ok := a.Config.Get(key)
	if !ok {
		return false, nil
	}
	if isNumericValue(flag.Value) {
		// Numeric flags read YAML's numeric value: 0x1F is 31 and 0755 is
		// 493, as in YAML; every other flag gets the text as written.
		if n, ok := a.Config.number(key); ok {
			val = n
		}
	}
	if err := setAndValidate(flag, val); err != nil {
		return false, fmt.Errorf("invalid value %q for --%s from config file key %q: %w", val, flag.Name, key, err)
	}
	flag.set = true
	flag.source = SourceConfigFile
	return true, nil
}

// isNumericValue reports whether v is one of the built-in int or float
// values.
func isNumericValue(v Value) bool {
	switch v.(type) {
	case *IntValue, *Int64Value, *Float64Value:
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

// missingFlagsError describes missing flags together with the environment
// variables that could supply them.
func (a *App) missingFlagsError(missing []*Flag) *MissingFlagsError {
	err := &MissingFlagsError{
		Flags:   make([]string, len(missing)),
		EnvVars: make([][]string, len(missing)),
	}
	for i, f := range missing {
		err.Flags[i] = f.Name
		err.EnvVars[i] = a.flagEnvVars(f)
	}
	return err
}

// promptForRequiredFlags interactively prompts for each missing required flag.
//
// Each answer is checked with the flag's parser and Validate function while
// the prompt is still open, so prompters that support validation (such as
// TextPrompter) re-prompt after a typo instead of aborting. An answer that
// is still invalid when the prompter returns is reported as an error naming
// the flag.
func (a *App) promptForRequiredFlags(ctx context.Context, cmd *Command, missing []*Flag) error {
	for _, flag := range missing {
		label := flag.Prompt
		if label == "" {
			label = strings.ReplaceAll(flag.Name, "-", " ")
			if len(label) > 0 {
				label = strings.ToUpper(label[:1]) + label[1:]
			}
		}

		value, err := a.prompter().Prompt(ctx, PromptRequest{
			Label:    label,
			Theme:    a.DefaultTheme,
			Validate: promptValidator(flag),
		})
		if err != nil {
			if errors.Is(err, io.EOF) {
				return fmt.Errorf("no input for required flag --%s: %w", flag.Name, err)
			}
			return err
		}
		if err := setAndValidate(flag, value); err != nil {
			return fmt.Errorf("invalid value for --%s: %w", flag.Name, err)
		}
		flag.set = true
		flag.source = SourcePrompt
	}
	return nil
}

// promptValidator returns a validation function that accepts exactly the
// answers flag would accept: the value must parse for the flag's type and
// then pass the flag's own Validate function.
func promptValidator(flag *Flag) func(string) error {
	return func(value string) error {
		if err := checkValue(flag.Value, value); err != nil {
			return err
		}
		if flag.Validate != nil {
			return flag.Validate(value)
		}
		return nil
	}
}

// checkValue reports whether raw parses for the built-in value types,
// without changing the flag. Custom Value implementations are checked when
// the answer is applied.
func checkValue(v Value, raw string) error {
	var err error
	switch v.(type) {
	case *BoolValue:
		_, err = strconv.ParseBool(raw)
	case *IntValue:
		_, err = strconv.ParseInt(raw, 10, 0)
	case *Int64Value:
		_, err = strconv.ParseInt(raw, 10, 64)
	case *Float64Value:
		_, err = strconv.ParseFloat(raw, 64)
	case *DurationValue:
		_, err = time.ParseDuration(raw)
	}
	return err
}

func (a *App) printCommandHelp(cmd *Command) error {
	helper := HelpRenderer{App: a, Command: cmd}
	return helper.Render(a.stdout())
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
