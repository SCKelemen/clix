package clix

import (
	"fmt"
	"sort"
	"strings"
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
type Command struct {
	Name      string
	Aliases   []string
	Short     string
	Long      string
	Usage     string
	Example   string
	Hidden    bool
	Flags     *FlagSet
	Arguments []*Argument
	Children  []*Command // Children of this command (groups or commands)

	Run     Handler
	PreRun  Hook
	PostRun Hook

	parent *Command
}

// NewCommand constructs a Command with an initialised flag set.
// This creates an executable command (leaf node) that can have a Run handler.
func NewCommand(name string) *Command {
	cmd := &Command{
		Name:  name,
		Flags: NewFlagSet(name),
	}

	cmd.Flags.BoolVar(&BoolVarOptions{
		Name:  "help",
		Short: "h",
		Usage: "Show help information",
	})

	return cmd
}

// NewGroup constructs a Command that acts as a group (interior node).
// Groups organize child commands but do not execute (no Run handler).
// Groups are used to create hierarchical command structures.
func NewGroup(name, short string, children ...*Command) *Command {
	cmd := &Command{
		Name:     name,
		Short:    short,
		Children: children,
		Flags:    NewFlagSet(name),
	}

	cmd.Flags.BoolVar(&BoolVarOptions{
		Name:  "help",
		Short: "h",
		Usage: "Show help information",
	})

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
		c.Flags.BoolVar(&BoolVarOptions{
			Name:  "help",
			Short: "h",
			Usage: "Show help information",
		})
	}

	for _, child := range c.Children {
		if child != nil {
			child.prepare(c)
		}
	}
}

// Path returns the command path from the root.
func (c *Command) Path() string {
	if c.parent == nil {
		return c.Name
	}
	return fmt.Sprintf("%s %s", c.parent.Path(), c.Name)
}

// RequiredArgs returns the number of required positional arguments.
func (c *Command) RequiredArgs() int {
	count := 0
	for _, arg := range c.Arguments {
		if arg.Required {
			count++
		}
	}
	return count
}

// findChild returns the first matching child command or group by name or alias.
func (c *Command) findChild(name string) *Command {
	name = strings.ToLower(name)
	for _, child := range c.Children {
		if strings.EqualFold(child.Name, name) {
			return child
		}
		for _, alias := range child.Aliases {
			if strings.EqualFold(alias, name) {
				return child
			}
		}
	}
	return nil
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

// VisibleChildren returns a sorted slice of child commands and groups that are not hidden.
func (c *Command) VisibleChildren() []*Command {
	var cmds []*Command
	for _, child := range c.Children {
		if child.Hidden {
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
