package prompt

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/internal/answer"
	"github.com/SCKelemen/clix/v2/internal/tty"
)

// linePrompt draws prompts one line at a time and reads typed lines. It is
// used whenever raw mode is not possible: input or output is not a
// terminal, TERM is "dumb", or the console cannot do virtual terminal
// sequences. Key bindings and command handlers need raw mode and are not
// used here; every line typed is an answer (a survey with an undo stack
// recognizes "back" itself).
//
// An invalid answer is reported and the next line is read. When the input
// ends right after an invalid answer, the prompt returns that validation
// error (it also matches io.EOF) instead of a bare io.EOF.
type linePrompt struct {
	in    io.Reader
	out   io.Writer
	color bool
}

func (p TerminalPrompter) linePrompt() *linePrompt {
	return &linePrompt{in: p.In, out: p.Out, color: tty.ColorEnabled(p.Out)}
}

func (l *linePrompt) style(style clix.TextStyle, text string) string {
	if style == nil {
		return text
	}
	out := style.Render(text)
	if !l.color {
		out = tty.StripANSI(out)
	}
	return out
}

func (l *linePrompt) header(cfg *clix.PromptConfig) string {
	return l.style(cfg.Theme.PrefixStyle, cfg.Theme.Prefix) + l.style(cfg.Theme.LabelStyle, cfg.Label)
}

func (l *linePrompt) printError(cfg *clix.PromptConfig, msg string) {
	if msg != "" {
		msg = l.style(cfg.Theme.ErrorStyle, msg)
	}
	_, _ = fmt.Fprintf(l.out, "%s%s\n", l.style(cfg.Theme.ErrorStyle, cfg.Theme.Error), msg)
}

// readLine reads one answer; secrets are read with echo off when the input
// is a terminal, and are not trimmed.
func (l *linePrompt) readLine(ctx context.Context, secret bool) (string, error) {
	if secret {
		if f, ok := l.in.(*os.File); ok && tty.IsTerminal(f) {
			if restore, err := tty.DisableEcho(f); err == nil {
				line, err := clix.ReadLine(ctx, l.in)
				restore()
				_, _ = fmt.Fprint(l.out, "\n")
				return line, err
			}
		}
		return clix.ReadLine(ctx, l.in)
	}
	line, err := clix.ReadLine(ctx, l.in)
	return strings.TrimSpace(line), err
}

// text prompts for free text: "prefix label [default] hint: ".
func (l *linePrompt) text(ctx context.Context, cfg *clix.PromptConfig) (string, error) {
	var invalid error
	for {
		_, _ = fmt.Fprint(l.out, l.header(cfg))
		if cfg.Default != "" && !cfg.Secret {
			_, _ = fmt.Fprintf(l.out, " [%s]", l.style(placeholderStyle(cfg.Theme), cfg.Default))
		} else if cfg.Default == "" && cfg.NoDefaultPlaceholder != "" {
			_, _ = fmt.Fprintf(l.out, " [%s]", l.style(placeholderStyle(cfg.Theme), cfg.NoDefaultPlaceholder))
		}
		if cfg.Theme.Hint != "" {
			_, _ = fmt.Fprintf(l.out, " %s", l.style(cfg.Theme.HintStyle, cfg.Theme.Hint))
		}
		_, _ = fmt.Fprint(l.out, ": ")

		line, err := l.readLine(ctx, cfg.Secret)
		if err != nil {
			return "", answer.Finish(err, invalid)
		}
		value, _, msg := submitText(cfg, line, false)
		if msg != "" {
			invalid = fmt.Errorf("%s", msg)
			l.printError(cfg, msg)
			continue
		}
		return value, nil
	}
}

// confirm prompts for yes or no: "prefix label (Y/n) hint: ".
func (l *linePrompt) confirm(ctx context.Context, cfg *clix.PromptConfig) (string, error) {
	var invalid error
	for {
		_, _ = fmt.Fprint(l.out, l.header(cfg))
		if answer.DefaultYes(cfg.Default) {
			_, _ = fmt.Fprint(l.out, " (Y/n)")
		} else {
			_, _ = fmt.Fprint(l.out, " (y/N)")
		}
		if cfg.Theme.Hint != "" {
			_, _ = fmt.Fprintf(l.out, " %s", l.style(cfg.Theme.HintStyle, cfg.Theme.Hint))
		}
		_, _ = fmt.Fprint(l.out, ": ")

		line, err := l.readLine(ctx, false)
		if err != nil {
			return "", answer.Finish(err, invalid)
		}
		value, _, msg := submitText(cfg, line, true)
		if msg != "" {
			invalid = fmt.Errorf("%s", msg)
			l.printError(cfg, msg)
			continue
		}
		return value, nil
	}
}

func (l *linePrompt) printOptions(cfg *clix.PromptConfig, marker func(i int) string, numbered bool) {
	_, _ = fmt.Fprint(l.out, l.header(cfg))
	if cfg.Theme.Hint != "" {
		_, _ = fmt.Fprintf(l.out, " %s", l.style(cfg.Theme.HintStyle, cfg.Theme.Hint))
	}
	_, _ = fmt.Fprint(l.out, "\n")
	for i, opt := range cfg.Options {
		if numbered {
			_, _ = fmt.Fprintf(l.out, "%s %d. %s", marker(i), i+1, opt.Label)
		} else {
			_, _ = fmt.Fprintf(l.out, "%s %s", marker(i), opt.Label)
		}
		if opt.Description != "" {
			_, _ = fmt.Fprintf(l.out, " - %s", opt.Description)
		}
		_, _ = fmt.Fprint(l.out, "\n")
	}
	_, _ = fmt.Fprint(l.out, "> ")
}

// choose prompts for one option. An answer is matched against the options
// in this order: the exact value or label (ignoring case), then the
// option's number when the whole answer is a number, then a prefix of
// exactly one label or value. Anything else is reported and asked again.
func (l *linePrompt) choose(ctx context.Context, cfg *clix.PromptConfig) (string, error) {
	def := singleDefault(cfg)
	var invalid error
	for {
		shown := def
		if shown < 0 {
			shown = 0
		}
		l.printOptions(cfg, func(i int) string {
			if i == shown {
				return ">"
			}
			return " "
		}, false)

		line, err := l.readLine(ctx, false)
		if err != nil {
			return "", answer.Finish(err, invalid)
		}

		var idx int
		var msg string
		if line == "" {
			idx = shown
		} else {
			idx, msg = matchOption(cfg.Options, line)
		}
		if msg == "" && cfg.Validate != nil {
			if err := cfg.Validate(cfg.Options[idx].Value); err != nil {
				msg = nonEmpty(err.Error())
			}
		}
		if msg != "" {
			invalid = fmt.Errorf("%s", msg)
			l.printError(cfg, msg)
			continue
		}
		return cfg.Options[idx].Value, nil
	}
}

// matchOption finds the option an answer names; msg explains a failure.
func matchOption(opts []clix.SelectOption, input string) (int, string) {
	for i, o := range opts {
		if strings.EqualFold(o.Value, input) || strings.EqualFold(o.Label, input) {
			return i, ""
		}
	}
	if n, ok := parseNumber(input); ok {
		if n >= 1 && n <= len(opts) {
			return n - 1, ""
		}
		return 0, fmt.Sprintf("no option %d; enter 1 to %d", n, len(opts))
	}
	lower := strings.ToLower(input)
	match := -1
	var names []string
	for i, o := range opts {
		if strings.HasPrefix(strings.ToLower(o.Label), lower) || strings.HasPrefix(strings.ToLower(o.Value), lower) {
			if match < 0 {
				match = i
			}
			names = append(names, o.Label)
		}
	}
	switch len(names) {
	case 0:
		return 0, fmt.Sprintf("%q matches no option", input)
	case 1:
		return match, ""
	default:
		return 0, fmt.Sprintf("%q matches more than one option: %s", input, strings.Join(names, ", "))
	}
}

// parseNumber accepts an answer made only of digits (and surrounding
// spaces), so "3rd-party" is not option 3.
func parseNumber(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}

// chooseMany prompts for several options. Each line toggles options by
// number ("1,3" or "1 3") or by exact value or label; "done", "finish", "q"
// or an empty line finishes. Finishing with nothing selected is refused
// unless Validate accepts the empty answer.
func (l *linePrompt) chooseMany(ctx context.Context, cfg *clix.PromptConfig) (string, error) {
	selected := map[int]bool{}
	for _, i := range multiDefault(cfg) {
		selected[i] = true
	}
	var invalid error
	fail := func(msg string) {
		invalid = fmt.Errorf("%s", msg)
		l.printError(cfg, msg)
	}
	for {
		l.printOptions(cfg, func(i int) string {
			if selected[i] {
				return "[x]"
			}
			return "[ ]"
		}, true)

		line, err := l.readLine(ctx, false)
		if err != nil {
			return "", answer.Finish(err, invalid)
		}

		lower := strings.ToLower(line)
		if line == "" || lower == "done" || lower == "finish" || lower == "q" {
			value := formatSelected(cfg.Options, selected)
			if cfg.Validate != nil {
				if err := cfg.Validate(value); err != nil {
					fail(nonEmpty(err.Error()))
					continue
				}
			} else if countSelected(selected) == 0 {
				fail("Please select at least one option")
				continue
			}
			return value, nil
		}

		if idx, ok := parseNumberList(line, len(cfg.Options)); ok {
			for _, i := range idx {
				selected[i] = !selected[i]
			}
			invalid = nil
			continue
		}
		found := false
		for i, o := range cfg.Options {
			if strings.EqualFold(o.Value, line) || strings.EqualFold(o.Label, line) {
				selected[i] = !selected[i]
				found = true
				break
			}
		}
		if !found {
			fail("Invalid selection. Enter option numbers (e.g., 1,2,3)")
			continue
		}
		invalid = nil
	}
}

// parseNumberList parses "1,3", "1 3" or "1, 3" as 0-based indices. Every
// part must be a number in range.
func parseNumberList(input string, n int) ([]int, bool) {
	parts := strings.Fields(strings.ReplaceAll(input, ",", " "))
	if len(parts) == 0 {
		return nil, false
	}
	idx := make([]int, 0, len(parts))
	for _, part := range parts {
		v, ok := parseNumber(part)
		if !ok || v < 1 || v > n {
			return nil, false
		}
		idx = append(idx, v-1)
	}
	return idx, true
}
