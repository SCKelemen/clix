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
// If args is nil, os.Args[1:] is used. Under go test those are the test
// binary's own flags (-test.v and so on), so tests should always pass an
// explicit slice, such as []string{} for no arguments. Main is the usual
// entry point of a program: it calls Run with os.Args[1:], handles
// signals, prints the error and exits.
// The context is propagated to command handlers and can be used for cancellation.
//
// Run is not safe for concurrent use on the same App: it resets and fills
// the flags of the command tree. Calling Run several times in sequence is
// supported: each Run first undoes what the previous Run set in every flag
// and its destination variable, so values parsed by one Run never leak
// into the next, while a value the program assigned between two Runs is
// kept. Custom values need a Reset method for this (see FlagSet.Var). A
// handler may call Run itself (an alias command, an interactive shell):
// that nested Run starts from the values the outer Run has, and when it
// returns it puts back everything it changed, so the handler continues
// with its own values, Changed and Source. To run commands in parallel,
// build one App per goroutine.
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
	// --help and --version have had a chance to run. With named
	// configurations the file depends on --configuration, so it is loaded
	// after parsing instead.
	var configErr error
	if !a.configurations {
		configErr = a.ensureConfigLoaded(ctx)
	}

	rootFlags := a.Root.Flags

	// Undo what the previous Run set (unless the program changed it
	// since), and record what this one changes. A nested Run (started by
	// a handler) puts back what it changed when it ends.
	end := a.startRun()
	defer end()

	// Find the command first, then parse every flag exactly once: flags bind
	// to the innermost command that defines them and fall back to the root.
	path, positionals, unknown, stray, err := a.parseCommandLine(args)
	if err != nil {
		return err
	}
	cmd := path[len(path)-1]
	if a.configurations {
		configErr = a.ensureConfigLoaded(ctx)
	}
	if cmd != a.Root {
		// Root flags given while another command runs are app flags.
		for _, f := range rootFlags.flags {
			if f.cliSet {
				f.source = SourceAppFlag
			}
		}
	}

	// Check if global --version flag was set
	if version, _ := rootFlags.Bool("version"); version {
		// Show version info - same format as "version" command but simpler (no commit/date)
		// The version extension sets app.Version, so if it's enabled, we show it
		if a.Version != "" {
			_, _ = fmt.Fprintf(a.stdout(), "%s version %s\n", a.Name, a.Version)
		} else {
			_, _ = fmt.Fprintf(a.stdout(), "%s\n", a.Name)
		}
		return nil
	}

	// --help/-h (automatic for all commands) takes precedence over
	// everything else, including positional arguments and required flags.
	if helpRequested(path) {
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
			// An unknown flag the non-strict root let through is still an
			// unknown flag, not just an extra argument.
			for i := len(positionals) - len(excess); i < len(positionals); i++ {
				if unknown[i] {
					name, _, _ := strings.Cut(positionals[i], "=")
					return usageErrorf("unknown flag: %s", name)
				}
			}
			if !cmd.Flags.takeRest(excess) {
				return usageErrorf("unexpected arguments: %s", strings.Join(excess, " "))
			}
		}
	}

	// A command with children:
	// - with a Run handler, runs it (a command with children can have
	//   default behavior);
	// - without one, is a group and shows help. This includes a root whose
	//   only children were added by extensions (help, version, ...).
	if len(cmd.Children) > 0 && cmd.Run == nil {
		return a.printCommandHelp(cmd)
	}

	// Flag groups (see FlagSet.MarkMutuallyExclusive): two flags of a
	// mutually exclusive group on the command line are a usage error.
	groups := pathGroups(path)
	if err := checkExclusiveCommandLine(groups); err != nil {
		return err
	}

	if configErr != nil && !a.configErrorAllowed(cmd) {
		return configErr
	}

	// Fill in flags the command line did not set, in precedence order:
	// environment, then config file, then defaults. Every command on the
	// path takes part: the root, each group, and the command itself. A
	// mutually exclusive group is filled like one setting.
	if err := a.planGroupFill(groups, configErrorsAllowed(cmd)); err != nil {
		return err
	}
	for _, c := range path {
		if err := a.applyConfigToFlags(c.Flags, configErrorsAllowed(cmd)); err != nil {
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
	// Ambient flags never count: "--target X widgets create" only says
	// where to create the widget, so it still prompts for the widget.
	missing := missingRequired(path)
	if len(missing) > 0 {
		passed := false
		for _, c := range path {
			passed = passed || c.Flags.anyInputSet()
		}
		if passed || !a.IsInteractive() {
			// Mode 3 (or nobody to ask): report what is missing.
			return a.missingFlagsError(missing)
		}
	}
	// A broken group requirement is reported before any prompt; the flags
	// about to be prompted for count as given.
	if err := a.checkGroupRequirements(groups, missing); err != nil {
		return err
	}
	if len(missing) > 0 {
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
		path:    path,
	}

	err = runHandlers(runCtx, cmd)
	// A progress indicator the handlers left running ends with them, so
	// it never keeps drawing after Run returns.
	a.endProgress(runCtx, err)
	return err
}

// runHandlers runs cmd's PreRun, Run and PostRun handlers in order,
// stopping at the first error.
func runHandlers(runCtx *Context, cmd *Command) error {
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
// in a single pass. README.md ("Routing") states these rules for users.
//
// The path is found first: words that name a child command descend into it,
// and flag tokens are skipped (together with their value, when the flag is
// known to take one). Parsing then binds each flag to the command in effect
// where it appears: the flag is looked up on that command, then on each of
// its ancestors up to the root, so a command's -f is never shadowed by a
// root -f, and a group's flag also works after the leaf. A bare "--" ends
// flag parsing and everything after it is positional.
//
// It returns the matched command path, the positional arguments, and the
// first word that did not match a child command (stray), if any.
//
// unknown marks the positionals that are unknown flags a non-strict flag
// set let through.
func (a *App) parseCommandLine(args []string) (path []*Command, positionals []string, unknown map[int]bool, stray string, err error) {
	root := a.Root
	path = []*Command{root}
	scope := make([]int, len(args))    // depth on path of the command in effect for each flag token, plus one
	isPath := make([]bool, len(args))  // tokens that name the command path
	isValue := make([]bool, len(args)) // tokens pass 1 skipped as a flag's value
	scanned := len(args)               // pass 1 looked at args[:scanned]
	sawWord := false

	// Pass 1: find the command path.
scan:
	for i := 0; i < len(args); i++ {
		scanned = i + 1
		tok := args[i]
		switch {
		case tok == "--":
			break scan
		case isFlagArg(tok):
			uses, ok := resolveFlagToken(tok, flagChain(path), nil)
			if !ok && isNumberArg(tok) && !a.mayBeFlag(tok) {
				// "-5" is a word, like any other positional, unless some
				// command registers it as a flag (-1 for --one), which a
				// command named later may own.
				stray = tok
				break scan
			}
			scope[i] = len(path)
			if ok && uses[len(uses)-1].needsNext && i+1 < len(args) {
				i++ // skip the flag's value
				isValue[i] = true
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
			child := path[len(path)-1].findChildMatch(tok, a.CaseSensitive)
			if child == nil {
				stray = tok
				break scan
			}
			isPath[i] = true
			path = append(path, child)
		}
	}

	// A command may be shared by several groups; its parent is the one on
	// the path that was matched, so Path and help name that path.
	for i := 1; i < len(path); i++ {
		path[i].parent = path[i-1]
	}
	markShadowed(path)
	cmd := path[len(path)-1]

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

		depth := scope[i]
		if depth == 0 {
			depth = len(path)
		}
		// A switch written before the command name, such as
		// "app --force delete", can still belong to the command (late).
		// Only a flag that does not need a separate value is accepted that
		// way, because pass 1 could not have known to skip the value.
		var late []*FlagSet
		if depth != len(path) {
			late = flagChain(path)
		}
		uses, ok := resolveFlagToken(tok, flagChain(path[:depth]), late)
		if !ok {
			switch {
			case isNumberArg(tok):
				positionals = append(positionals, tok)
			case cmd.Flags.strict:
				return nil, nil, nil, "", usageErrorf("unknown flag: %s", unknownFlagName(tok, flagChain(path[:depth]), late))
			default:
				if unknown == nil {
					unknown = make(map[int]bool)
				}
				unknown[len(positionals)] = true
				positionals = append(positionals, tok)
			}
			continue
		}
		for _, u := range uses {
			// Both passes must agree on which tokens are values. A cluster
			// written before the command, such as -fp in "myapp -fp deploy"
			// with -f from deploy and -p from the root, could not be read
			// in pass 1, so the next token was not set aside as -p's value
			// and may be the command itself.
			if u.needsNext && i+1 < scanned && !isValue[i+1] {
				return nil, nil, nil, "", usageErrorf("%s needs a value: write %s=VALUE, or put the flags after the command", u.name, tok)
			}
			err := setFromArg(u.flag, u.name, u.value, u.hasValue, func() (string, bool) {
				if i+1 >= len(args) {
					return "", false
				}
				i++
				return args[i], true
			})
			if err != nil {
				return nil, nil, nil, "", err
			}
		}
	}

	return path, positionals, unknown, stray, nil
}

// mayBeFlag reports whether some command in the tree registers tok's name
// as a flag, or every letter of it as a short flag.
func (a *App) mayBeFlag(tok string) bool {
	name, _, _ := strings.Cut(tok, "=")
	found := false
	eachFlagSetOnce(a.Root, func(fs *FlagSet) {
		if found || fs.index[name] != nil {
			found = true
			return
		}
	})
	if found {
		return true
	}
	for _, r := range name[1:] {
		short := "-" + string(r)
		has := false
		eachFlagSetOnce(a.Root, func(fs *FlagSet) { has = has || fs.index[short] != nil })
		if !has {
			return false
		}
	}
	return true
}

// unknownFlagName names an unknown flag token in an error: the letter that
// is not a flag when tok is a cluster whose first letter is one ("-z (in
// -fz)"), and otherwise the token up to any "=".
func unknownFlagName(tok string, sets, late []*FlagSet) string {
	name, _, _ := strings.Cut(tok, "=")
	if len(name) < 3 || name[1] == '-' {
		return name
	}
	for i, r := range name[1:] {
		short := "-" + string(r)
		if lookupArg(short, sets...) == nil && lookupArg(short, late...) == nil {
			if i == 0 {
				return name
			}
			return short + " (in " + name + ")"
		}
	}
	return name
}

// flagChain returns the flag sets of the commands on path, innermost
// first, ending with the root's.
func flagChain(path []*Command) []*FlagSet {
	sets := make([]*FlagSet, 0, len(path))
	for i := len(path) - 1; i >= 0; i-- {
		if path[i].Flags != nil {
			sets = append(sets, path[i].Flags)
		}
	}
	return sets
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

// missingRequired returns the required flags without a value on path
// (root first), outermost command first, leaving out the flags the Run
// ignores (see markShadowed). A command added by an extension
// (IsExtensionCommand), and everything below it, only checks the flags
// from that command down, so "app version" never asks for a required
// --project on the root.
func missingRequired(path []*Command) []*Flag {
	from := extensionFrom(path)
	var missing []*Flag
	for _, c := range path[from:] {
		for _, f := range c.Flags.MissingRequired() {
			if !f.shadowed {
				missing = append(missing, f)
			}
		}
	}
	return missing
}

// helpRequested reports whether --help was passed for any command on
// path, including the root.
func helpRequested(path []*Command) bool {
	for _, c := range path {
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
// set and a --help flag. It is idempotent, so Run calls it every time. A
// command shared by several groups keeps the parent it was last reached
// through (by Run or ResolvePath) while that group still lists it, so
// preparing never changes what Path reports.
func (a *App) prepareTree() {
	if a.Root == nil {
		return
	}
	a.Root.parent = nil
	listedBy := map[*Command][]*Command{}
	eachCommandOnce(a.Root, func(c *Command) {
		c.caseSensitive = a.CaseSensitive
		c.prepareFlags()
		for _, child := range c.Children {
			if child != nil && child != a.Root {
				listedBy[child] = append(listedBy[child], c)
			}
		}
	})
	for child, parents := range listedBy {
		listed := false
		for _, p := range parents {
			listed = listed || p == child.parent
		}
		if !listed {
			child.parent = parents[len(parents)-1]
		}
	}
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

// ensureConfigLoaded loads the config file once. With named
// configurations it loads again when another configuration is selected,
// dropping the values of the previous one first.
func (a *App) ensureConfigLoaded(ctx context.Context) error {
	path, err := a.configFilePath()
	var selection *configurationError
	if errors.As(err, &selection) {
		// No configuration's values apply, not even the last one loaded.
		if a.configLoaded && a.Config != nil {
			a.Config.Reset()
		}
		a.configLoaded, a.configLoadedPath, a.configLoadErr = false, "", nil
		return selection.err
	}
	if a.configLoaded && (path == a.configLoadedPath || !a.configurations) {
		// Without named configurations the file is loaded once per App,
		// as it always was.
		return a.configLoadErr
	}
	if a.Config == nil {
		a.Config = NewConfigManager(a.Name)
	} else if a.configLoaded {
		a.Config.Reset()
	}
	a.configLoaded, a.configLoadedPath = true, path

	if err != nil {
		// Without a home directory there is no config file to read; that is
		// not an error for commands that do not save configuration.
		a.configLoadErr = nil
		return nil
	}
	a.configLoadErr = a.Config.Load(path)
	return a.configLoadErr
}

// applyConfigToFlags fills in every flag the command line did not set,
// trying environment variables, then the config file, then the flag's
// default. It must run after parsing so command-line values keep the highest
// precedence. A value that the flag rejects is reported as an error naming
// its source. Flags the Run ignores (see markShadowed) are left alone.
func (a *App) applyConfigToFlags(flags *FlagSet, tolerate bool) error {
	if flags == nil {
		return nil
	}

	for _, flag := range flags.flags {
		if flag.cliSet || flag.shadowed {
			continue
		}

		// CommandLineOnly flags, and the built-in --help and --version
		// switches, only come from the command line; MYAPP_FORCE or
		// MYAPP_HELP must not hijack every run.
		if isCommandLineOnly(flag) {
			if err := a.trySetFromDefault(flag); err != nil {
				return err
			}
			continue
		}

		// Try each source in order of precedence, except those a mutually
		// exclusive group rules out (see planGroupFill).
		undo := valueUndo(flag, tolerate)
		if flag.groupSkip&skipEnv == 0 {
			if ok, err := a.trySetFromEnv(flag); ok || err != nil {
				if err := a.valueError(flag, err, tolerate, undo); err != nil {
					return err
				}
				continue
			}
		}
		if flag.groupSkip&skipConfig != 0 {
			if err := a.trySetFromDefault(flag); err != nil {
				return err
			}
			continue
		}
		if ok, err := a.trySetFromConfig(flag); ok || err != nil {
			if err := a.valueError(flag, err, tolerate, undo); err != nil {
				return err
			}
			continue
		}
		if err := a.trySetFromDefault(flag); err != nil {
			return err
		}
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
	return switchOf(flag.Value) != nil
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
		// A usage error, like the same value on the command line.
		return false, usageErrorf("invalid value %s for --%s from environment variable %s: %w", shownValue(flag, val), flag.Name, name, redactErr(flag, err, val))
	}
	flag.set = true
	flag.source = SourceEnvVar
	return true, nil
}

// setAndValidate parses raw into flag and then runs the flag's Validate
// function, mirroring what the command-line parser does.
func setAndValidate(flag *Flag, raw string) error {
	if err := flag.setValue(raw); err != nil {
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
	if slice, isSlice := flag.Value.(*StringSliceValue); isSlice {
		// A YAML list goes to a slice flag item by item, so an item may
		// contain commas or quotes.
		if items, ok := a.Config.list(key); ok {
			slice.setItems(items)
			if flag.Validate != nil {
				if err := flag.Validate(slice.String()); err != nil {
					return false, usageErrorf("invalid value %s for --%s from config file key %q: %w", shownValue(flag, slice.String()), flag.Name, key, redactErr(flag, err, slice.String()))
				}
			}
			flag.set = true
			flag.source = SourceConfigFile
			return true, nil
		}
	}
	if isNumericValue(flag.Value) {
		// Numeric flags read YAML's numeric value: 0x1F is 31 and 0755 is
		// 493, as in YAML; every other flag gets the text as written.
		if n, ok := a.Config.number(key); ok {
			val = n
		}
	}
	if err := setAndValidate(flag, val); err != nil {
		return false, usageErrorf("invalid value %s for --%s from config file key %q: %w", shownValue(flag, val), flag.Name, key, redactErr(flag, err, val))
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

// trySetFromDefault sets a flag value from its default if available. A
// default the flag rejects (one changed after registration, say) is an
// error.
func (a *App) trySetFromDefault(flag *Flag) error {
	if !flag.hasDefault() {
		return nil
	}
	if err := flag.setValue(flag.Default); err != nil {
		return fmt.Errorf("clix: invalid default %q for --%s: %w", flag.Default, flag.Name, err)
	}
	return nil
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
			Secret:   flag.Sensitive,
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
	return flag.CheckValue
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
