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

// Command represents a CLI command. Commands can contain nested subcommands,
// flags, argument definitions and execution hooks.
type Command struct {
	Name        string
	Aliases     []string
	Short       string
	Long        string
	Usage       string
	Example     string
	Hidden      bool
	Flags       *FlagSet
	Arguments   []*Argument
	Subcommands []*Command

	Run     Handler
	PreRun  Hook
	PostRun Hook

	parent *Command
}

// NewCommand constructs a Command with an initialised flag set.
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

// AddCommand registers a subcommand. The parent/child relationship is managed
// automatically.
func (c *Command) AddCommand(cmd *Command) {
	cmd.prepare(c)
	c.Subcommands = append(c.Subcommands, cmd)
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

	for _, sub := range c.Subcommands {
		if sub != nil {
			sub.prepare(c)
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

// findSubcommand returns the first matching subcommand by name or alias.
func (c *Command) findSubcommand(name string) *Command {
	name = strings.ToLower(name)
	for _, sub := range c.Subcommands {
		if strings.EqualFold(sub.Name, name) {
			return sub
		}
		for _, alias := range sub.Aliases {
			if strings.EqualFold(alias, name) {
				return sub
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
		next := current.findSubcommand(rest[0])
		if next == nil {
			break
		}
		rest = rest[1:]
		current = next
	}

	return current, rest
}

// VisibleSubcommands returns a sorted slice of subcommands that are not hidden.
func (c *Command) VisibleSubcommands() []*Command {
	var cmds []*Command
	for _, sub := range c.Subcommands {
		if sub.Hidden {
			continue
		}
		cmds = append(cmds, sub)
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
		next := cmd.findSubcommand(part)
		if next == nil {
			return nil
		}
		cmd = next
	}
	return cmd
}
