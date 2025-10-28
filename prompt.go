package clix

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Prompter encapsulates interactive prompting.
type Prompter interface {
	Prompt(ctx context.Context, req PromptRequest) (string, error)
}

// PromptRequest carries the information necessary to display a prompt.
type PromptRequest struct {
	Label    string
	Default  string
	Validate func(string) error
	Theme    PromptTheme
}

// PromptTheme defines how prompts are styled.
type PromptTheme struct {
	Prefix string
	Hint   string
	Error  string
}

// DefaultPromptTheme provides a sensible default for terminal prompts.
var DefaultPromptTheme = PromptTheme{
	Prefix: "? ",
	Hint:   "",
	Error:  "! ",
}

// TerminalPrompter implements Prompter using standard input/output streams.
type TerminalPrompter struct {
	In  io.Reader
	Out io.Writer
}

// Prompt displays a prompt and reads the user's response.
func (p TerminalPrompter) Prompt(ctx context.Context, req PromptRequest) (string, error) {
	if p.In == nil || p.Out == nil {
		return "", errors.New("prompter missing IO")
	}

	reader := bufio.NewReader(p.In)

	for {
		fmt.Fprintf(p.Out, "%s%s", req.Theme.Prefix, req.Label)
		if req.Default != "" {
			fmt.Fprintf(p.Out, " [%s]", req.Default)
		}
		fmt.Fprint(p.Out, ": ")

		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		value := strings.TrimSpace(line)
		if value == "" {
			value = req.Default
		}

		if req.Validate != nil {
			if err := req.Validate(value); err != nil {
				fmt.Fprintf(p.Out, "%s%s\n", req.Theme.Error, err)
				continue
			}
		}

		return value, nil
	}
}
