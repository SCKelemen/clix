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

	PrefixStyle  TextStyle
	LabelStyle   TextStyle
	HintStyle    TextStyle
	DefaultStyle TextStyle
	ErrorStyle   TextStyle
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
		prefix := renderText(req.Theme.PrefixStyle, req.Theme.Prefix)
		label := renderText(req.Theme.LabelStyle, req.Label)
		fmt.Fprintf(p.Out, "%s%s", prefix, label)

		if req.Default != "" {
			def := renderText(req.Theme.DefaultStyle, req.Default)
			fmt.Fprintf(p.Out, " [%s]", def)
		}

		if req.Theme.Hint != "" {
			hint := renderText(req.Theme.HintStyle, req.Theme.Hint)
			fmt.Fprintf(p.Out, " %s", hint)
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
				errPrefix := renderText(req.Theme.ErrorStyle, req.Theme.Error)
				errMsg := err.Error()
				if errMsg != "" {
					errMsg = renderText(req.Theme.ErrorStyle, errMsg)
				}
				fmt.Fprintf(p.Out, "%s%s\n", errPrefix, errMsg)
				continue
			}
		}

		return value, nil
	}
}
