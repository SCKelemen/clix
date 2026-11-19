package clix

import (
	"fmt"
	"sort"
)

// Handler is the function signature for executing a command.
type Handler func(ctx *Context) error

// Hook is executed before or after the main handler.
type Hook func(ctx *Context) error

// Command represents a CLI command. Commands can contain nested children
// (groups or commands), flags, argument definitions and execution hooks.
//
// A Command can be one of three types:
//   - A Group: has children but no Run handler (interior node, shows help when called)
//   - A Leaf Command: has a Run handler but no children (executable leaf node)
//   - A Command with Children: has both a Run handler and children
//     (executes Run handler when called without args, or routes to child commands
//     when a child name is provided)
//
// Example:
//
//	// Create a group (organizes child commands)
//	projectGroup := clix.NewGroup("project", "Manage projects",
//		clix.NewCommand("list", clix.WithCommandShort("List projects"), clix.WithCommandRun(listProjects)),
//		clix.NewCommand("get", clix.WithCommandShort("Get a project"), clix.WithCommandRun(getProject)),
//	)
//
//	// Create a command (executable)
//	helloCmd := clix.NewCommand("hello")
//	helloCmd.Short = "Say hello"
//	helloCmd.Run = func(ctx *clix.Context) error {
//		fmt.Println("Hello!")
//		return nil
//	}
type Command struct {
	// Name is the command name, used for matching and in help output.
	Name string

	// Aliases are alternative names for the command (e.g., ["ls", "list"]).
	Aliases []string

	// Short is a brief one-line description shown in command lists.
	Short string

	// Long is a detailed multi-line description shown in command help.
	Long string

	// Usage is a usage string shown in help output (e.g., "myapp cmd [flags] [args]").
	// If empty, a default usage string is generated.
	Usage string

	// Example shows example usage in help output.
	Example string

	// Hidden hides the command from help output and autocomplete.
	Hidden bool

	// IsExtensionCommand indicates this command was added by an extension.
	// Extension commands are not counted when determining if a command has user-defined children.
	IsExtensionCommand bool

	// Flags is the flag set for this command. Flags defined here are scoped to this command.
	// Use app.Flags() for flags that apply to all commands.
	Flags *FlagSet

	// Children are the child commands or groups of this command.
	// Use NewGroup() to create groups, NewCommand() to create executable commands.
	Children []*Command

	// Run is the handler executed when this command is invoked.
	// If the command has children and no Run handler, it shows help (group behavior).
	// If the command has both children and a Run handler, the Run handler executes
	// when called without matching child commands.
	Run Handler

	// PreRun is executed before Run. Useful for setup or validation.
	PreRun Hook

	// PostRun is executed after Run. Useful for cleanup or finalization.
	PostRun Hook

	parent        *Command
	owner              // the app, or the extension whose Extend added the command
	caseSensitive bool // App.CaseSensitive, copied by the App so name matching agrees everywhere
}

// CommandOption configures a command using the functional options pattern.
// Options can be used to build commands:
//
//	// Using functional options
//	cmd := clix.NewCommand("hello",
//		WithCommandShort("Say hello"),
//		WithCommandRun(func(ctx *clix.Context) error {
//			fmt.Println("Hello!")
//			return nil
//		}),
//	)
//
//	// Using struct (primary API)
//	cmd := clix.NewCommand("hello")
//	cmd.Short = "Say hello"
//	cmd.Run = func(ctx *clix.Context) error { ... }
type CommandOption interface {
	// ApplyCommand configures a command struct.
	// Exported so extension packages can implement CommandOption.
	ApplyCommand(*Command)
}

// NewCommand constructs a Command with an initialised flag set.
// This creates an executable command (leaf node) that can have a Run handler.
// Accepts optional CommandOption arguments for configuration.
//
// Example - three API styles:
//
//	// 1. Struct-based (primary API)
//	cmd := clix.NewCommand("hello")
//	cmd.Short = "Say hello"
//	cmd.Run = func(ctx *clix.Context) error {
//		fmt.Println("Hello!")
//		return nil
//	}
//
//	// 2. Functional options
//	cmd := clix.NewCommand("hello",
//		clix.WithCommandShort("Say hello"),
//		clix.WithCommandRun(func(ctx *clix.Context) error {
//			fmt.Println("Hello!")
//			return nil
//		}),
//	)
//
//	// 3. Mixed (constructor + struct fields)
//	cmd := clix.NewCommand("hello",
//		clix.WithCommandShort("Say hello"),
//	)
//	cmd.Run = func(ctx *clix.Context) error { ... }
func NewCommand(name string, opts ...CommandOption) *Command {
	cmd := &Command{
		Name:  name,
		Flags: NewFlagSet(name),
	}
	addHelpFlag(cmd.Flags)

	for _, opt := range opts {
		opt.ApplyCommand(cmd)
	}

	return cmd
}

// NewGroup constructs a Command that acts as a group (interior node).
// Groups organize child commands but do not execute (no Run handler).
// Groups are used to create hierarchical command structures.
// NewGroup takes the group's children, not CommandOption values; set other
// fields on the returned Command, or apply options to it yourself.
//
//	group := clix.NewGroup("project", "Manage projects",
//		clix.NewCommand("list", clix.WithCommandRun(listProjects)),
//		clix.NewCommand("get", clix.WithCommandRun(getProject)),
//	)
//	group.Long = "Detailed description..."
//
//	// Options can still be applied after construction:
//	clix.WithCommandAliases("projects").ApplyCommand(group)
func NewGroup(name, short string, children ...*Command) *Command {
	cmd := &Command{
		Name:     name,
		Short:    short,
		Children: children,
		Flags:    NewFlagSet(name),
	}
	addHelpFlag(cmd.Flags)

	return cmd
}

// AddCommand registers a child command or group. The parent/child relationship
// is managed automatically.
func (c *Command) AddCommand(cmd *Command) {
	cmd.prepare(c)
	c.Children = append(c.Children, cmd)
}

// IsGroup returns true if this command is a group (has children but no Run handler).
// Groups are interior nodes that organize child commands.
func (c *Command) IsGroup() bool {
	return c.Run == nil && len(c.Children) > 0
}

// IsLeaf returns true if this command is a leaf (has a Run handler).
// Leaf commands are executable commands, even if they have flags or arguments.
func (c *Command) IsLeaf() bool {
	return c.Run != nil
}

func (c *Command) prepare(parent *Command) {
	c.parent = parent

	if c.Flags == nil {
		c.Flags = NewFlagSet(c.Name)
	}

	if c.Flags.lookup("help") == nil {
		addHelpFlag(c.Flags)
	}

	for _, child := range c.Children {
		if child != nil {
			child.prepare(c)
		}
	}
}

// addHelpFlag registers the built-in --help switch on fs. It asks for -h,
// but gives it up to any other flag that wants it (such as --host), so
// help never takes a short name from a user's flag.
func addHelpFlag(fs *FlagSet) {
	flag := FlagOptions{
		Name:  "help",
		Short: "h",
		Usage: "Show help information",
	}.newFlag(&BoolValue{}, "")
	flag.yieldsShort = true
	fs.addFlag(flag)
}

// Path returns the command path from the root.
func (c *Command) Path() string {
	if c.parent == nil {
		return c.Name
	}
	return fmt.Sprintf("%s %s", c.parent.Path(), c.Name)
}

// findChild returns the matching child command or group by name or
// alias, ignoring ASCII case unless the App is CaseSensitive.
func (c *Command) findChild(name string) *Command {
	return c.findChildMatch(name, c.caseSensitive)
}

// findChildMatch returns the child whose name or alias matches name.
// Names match exactly when caseSensitive is set, and otherwise ignoring
// ASCII case only: "DELETE" matches "delete", but non-ASCII letters must
// match exactly, so "ſtatus" (U+017F) does not match "status" and a Kelvin
// sign does not match "k".
//
// When several children match, the app's command beats one added by an
// extension, and between extensions the one added first wins; between
// commands of the same owner the first in Children wins. A command added
// by an extension whose Name is also a name or alias of a better-owned
// sibling is shadowed: it is not reachable at all, and it is left out of
// VisibleChildren (help and completion).
func (c *Command) findChildMatch(name string, caseSensitive bool) *Command {
	match := nameMatcher(caseSensitive)
	var best *Command
	for _, child := range c.Children {
		if child == nil || !answersTo(child, name, match) {
			continue
		}
		if best != nil && best.rank <= child.rank {
			continue
		}
		if c.shadowedBy(child, match) != nil {
			continue
		}
		best = child
	}
	return best
}

// nameMatcher returns the comparison command routing uses.
func nameMatcher(caseSensitive bool) func(a, b string) bool {
	if caseSensitive {
		return func(a, b string) bool { return a == b }
	}
	return equalFoldASCII
}

// answersTo reports whether cmd has name as its Name or one of its Aliases.
func answersTo(cmd *Command, name string, match func(a, b string) bool) bool {
	if match(cmd.Name, name) {
		return true
	}
	for _, alias := range cmd.Aliases {
		if match(alias, name) {
			return true
		}
	}
	return false
}

// shadowedBy returns the sibling of child, owned by the app or by an
// extension added earlier than child's, that answers to child's Name, or
// nil. Such a child is unreachable.
func (c *Command) shadowedBy(child *Command, match func(a, b string) bool) *Command {
	var best *Command
	for _, other := range c.Children {
		if other == nil || other == child || other.rank >= child.rank {
			continue
		}
		if answersTo(other, child.Name, match) && (best == nil || other.rank < best.rank) {
			best = other
		}
	}
	return best
}

// equalFoldASCII reports whether a and b are equal when ASCII letters are
// compared without regard to case. Unlike strings.EqualFold it does not
// apply Unicode case folding, so every non-ASCII byte must match exactly.
func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if x == y {
			continue
		}
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}

// match walks the command tree and returns the deepest command that matches the
// provided arguments and the remaining arguments to parse for flags and
// positionals.
func (c *Command) match(args []string) (*Command, []string) {
	current := c
	rest := args

	for len(rest) > 0 {
		next := current.findChild(rest[0])
		if next == nil {
			break
		}
		rest = rest[1:]
		current = next
	}

	return current, rest
}

// VisibleChildren returns a sorted slice of child commands and groups that
// are not hidden and not shadowed by a command of a better owner (see
// findChildMatch).
func (c *Command) VisibleChildren() []*Command {
	var cmds []*Command
	for _, child := range c.Children {
		if child == nil || child.Hidden || c.shadowedBy(child, nameMatcher(c.caseSensitive)) != nil {
			continue
		}
		cmds = append(cmds, child)
	}
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].Name < cmds[j].Name
	})
	return cmds
}

// Groups returns the child commands that are groups (interior nodes).
func (c *Command) Groups() []*Command {
	var groups []*Command
	for _, child := range c.VisibleChildren() {
		if child.IsGroup() {
			groups = append(groups, child)
		}
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})
	return groups
}

// Commands returns the child commands that are executable (leaf nodes).
func (c *Command) Commands() []*Command {
	var cmds []*Command
	for _, child := range c.VisibleChildren() {
		if child.IsLeaf() {
			cmds = append(cmds, child)
		}
	}
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].Name < cmds[j].Name
	})
	return cmds
}

// ResolvePath resolves a path like ["config", "set"] into a command.
func (c *Command) ResolvePath(path []string) *Command {
	cmd := c
	for _, part := range path {
		next := cmd.findChild(part)
		if next == nil {
			return nil
		}
		cmd = next
	}
	return cmd
}

// Functional option helpers for commands

// WithCommandShort sets the command short description.
func WithCommandShort(short string) CommandOption {
	return commandShortOption(short)
}

// WithCommandLong sets the command long description.
func WithCommandLong(long string) CommandOption {
	return commandLongOption(long)
}

// WithCommandUsage sets the command usage string.
func WithCommandUsage(usage string) CommandOption {
	return commandUsageOption(usage)
}

// WithCommandExample sets the command example string.
func WithCommandExample(example string) CommandOption {
	return commandExampleOption(example)
}

// WithCommandAliases sets the command aliases.
func WithCommandAliases(aliases ...string) CommandOption {
	return commandAliasesOption(aliases)
}

// WithCommandHidden marks the command as hidden.
func WithCommandHidden(hidden bool) CommandOption {
	return commandHiddenOption(hidden)
}

// WithCommandRun sets the command run handler.
func WithCommandRun(run Handler) CommandOption {
	return commandRunOption{run: run}
}

// WithCommandPreRun sets the command pre-run hook.
func WithCommandPreRun(preRun Hook) CommandOption {
	return commandPreRunOption{preRun: preRun}
}

// WithCommandPostRun sets the command post-run hook.
func WithCommandPostRun(postRun Hook) CommandOption {
	return commandPostRunOption{postRun: postRun}
}

// Internal option types

type commandShortOption string

func (o commandShortOption) ApplyCommand(cmd *Command) {
	cmd.Short = string(o)
}

type commandLongOption string

func (o commandLongOption) ApplyCommand(cmd *Command) {
	cmd.Long = string(o)
}

type commandUsageOption string

func (o commandUsageOption) ApplyCommand(cmd *Command) {
	cmd.Usage = string(o)
}

type commandExampleOption string

func (o commandExampleOption) ApplyCommand(cmd *Command) {
	cmd.Example = string(o)
}

type commandAliasesOption []string

func (o commandAliasesOption) ApplyCommand(cmd *Command) {
	cmd.Aliases = []string(o)
}

type commandHiddenOption bool

func (o commandHiddenOption) ApplyCommand(cmd *Command) {
	cmd.Hidden = bool(o)
}

type commandRunOption struct {
	run Handler
}

func (o commandRunOption) ApplyCommand(cmd *Command) {
	cmd.Run = o.run
}

type commandPreRunOption struct {
	preRun Hook
}

func (o commandPreRunOption) ApplyCommand(cmd *Command) {
	cmd.PreRun = o.preRun
}

type commandPostRunOption struct {
	postRun Hook
}

func (o commandPostRunOption) ApplyCommand(cmd *Command) {
	cmd.PostRun = o.postRun
}
