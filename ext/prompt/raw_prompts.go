package prompt

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/internal/answer"
	"github.com/SCKelemen/clix/v2/internal/tty"
)

// textLine renders the input line of a text, secret or confirm prompt and
// returns it with the display column of the cursor.
func (s *rawSession) textLine(cfg *clix.PromptConfig, ed *editor, confirm bool, final bool) (string, int) {
	head := s.header(cfg)
	if confirm {
		if answer.DefaultYes(cfg.Default) {
			head += " (Y/n)"
		} else {
			head += " (y/N)"
		}
	}
	if cfg.Theme.Hint != "" && !final {
		head += " " + s.style(cfg.Theme.HintStyle, cfg.Theme.Hint)
	}
	head += ": "
	shown := ed.text
	if cfg.Secret {
		shown = ""
	}
	line := head + shown
	cursor := tty.VisibleWidth(head)
	if !cfg.Secret {
		cursor += tty.StringWidth(ed.text[:ed.cursor])
	}
	if final {
		return line, cursor
	}
	// Ghost text: the rest of the default, or the placeholder when there is
	// no default. Neither is ever returned unless accepted.
	if !confirm && ed.cursor == len(ed.text) {
		ghost := ""
		if !cfg.Secret {
			ghost = suggestionText(cfg, ed.text)
		}
		if ghost == "" && ed.text == "" && cfg.Default == "" {
			ghost = cfg.NoDefaultPlaceholder
		}
		if ghost != "" {
			line += s.style(suggestionStyle(cfg.Theme), ghost)
		}
	}
	return line, cursor
}

// text runs a text, secret or confirm prompt in raw mode.
func (s *rawSession) text(ctx context.Context, cfg *clix.PromptConfig, confirm bool) (string, error) {
	ed := &editor{}
	errMsg := ""
	done := func(display string) {
		line, _ := s.textLine(cfg, &editor{text: display, cursor: len(display)}, confirm, true)
		s.finish(line)
	}
	cancelled := func() {
		shown := ed.text
		if confirm {
			shown = ""
		}
		done(shown)
	}

	for {
		suggestion := ""
		if !confirm && !cfg.Secret {
			suggestion = suggestionText(cfg, ed.text)
		}
		line, col := s.textLine(cfg, ed, confirm, false)
		lines := []string{line}
		if errMsg != "" {
			lines = append(lines, s.errorLine(cfg, errMsg))
		}
		if hint := s.hintLine(cfg, clix.PromptKeyState{Input: ed.text, Default: cfg.Default, Suggestion: suggestion}); hint != "" {
			lines = append(lines, hint)
		}
		s.draw(lines, 0, col)

		ev, err := s.next(ctx)
		if errors.Is(err, errResized) {
			continue
		}
		if err != nil {
			cancelled()
			return "", err
		}
		if ev.isPaste {
			ed.insert(ev.paste)
			errMsg = ""
			continue
		}
		key := ev.key
		if key == KeyCtrlC {
			cancelled()
			return "", clix.ErrCancelled
		}
		if cmd, ok := commandFor(key); ok {
			state := clix.PromptKeyState{Command: cmd, Input: ed.text, Default: cfg.Default, Suggestion: suggestion}
			action := dispatchCommand(cfg, state, ed.set)
			if action.Exit {
				cancelled()
				return "", exitError(action)
			}
			if action.Handled {
				continue
			}
			switch key {
			case KeyEnter:
				value, display, msg := submitText(cfg, ed.text, confirm)
				if msg != "" {
					errMsg = msg
					continue
				}
				done(display)
				return value, nil
			case KeyTab:
				if !confirm && cfg.Default != "" {
					ed.set(cfg.Default)
				}
			case KeyEscape:
				ed.set("")
			}
			errMsg = ""
			continue
		}
		if ed.handle(key) {
			errMsg = ""
		}
	}
}

// submitText turns typed input into the prompt's answer. msg is the
// validation message when the answer is rejected; display is what the
// finished prompt shows.
func submitText(cfg *clix.PromptConfig, input string, confirm bool) (value, display, msg string) {
	if confirm {
		yes := answer.DefaultYes(cfg.Default)
		if strings.TrimSpace(input) != "" {
			var ok bool
			if yes, ok = answer.Answer(input); !ok {
				return "", "", "please enter 'y' or 'n'"
			}
		}
		value = answer.Normalize(yes)
		display = "no"
		if yes {
			display = "yes"
		}
	} else {
		value = input
		if !cfg.Secret {
			value = strings.TrimSpace(value)
		}
		if value == "" {
			value = cfg.Default
		}
		display = value
		if cfg.Secret {
			display = ""
		}
	}
	if cfg.Validate != nil {
		if err := cfg.Validate(value); err != nil {
			msg = err.Error()
			if msg == "" {
				msg = "invalid answer"
			}
			return "", "", msg
		}
	}
	return value, display, ""
}

// choose runs a select or multi-select prompt in raw mode. The list scrolls
// within a window that fits the terminal.
func (s *rawSession) choose(ctx context.Context, cfg *clix.PromptConfig, multi bool) (string, error) {
	opts := cfg.Options
	n := len(opts)
	selected := map[int]bool{}
	cur := 0
	if multi {
		for _, i := range multiDefault(cfg) {
			selected[i] = true
		}
		for i := 0; i < n; i++ {
			if selected[i] {
				cur = i
				break
			}
		}
	} else if i := singleDefault(cfg); i >= 0 {
		cur = i
	}
	continueIdx := n // multi-select only: the Continue button
	last := n - 1
	if multi {
		last = n
	}
	start := 0
	errMsg := ""
	s.hideCursor()

	head := s.header(cfg)
	if cfg.Theme.Hint != "" {
		head += " " + s.style(cfg.Theme.HintStyle, cfg.Theme.Hint)
	}
	continueText := cfg.ContinueText
	if continueText == "" {
		continueText = "Continue"
	}
	more := func(dir string, count int) string {
		if count <= 0 {
			return ""
		}
		return s.style(cfg.Theme.HintStyle, fmt.Sprintf("  %s %d more", dir, count))
	}
	pageSize := 1

	render := func() {
		hint := s.hintLine(cfg, clix.PromptKeyState{Default: cfg.Default})
		fixed := 1
		if multi {
			fixed++
		}
		if errMsg != "" {
			fixed++
		}
		if hint != "" {
			fixed++
		}
		avail := s.rows - fixed - 1
		if avail < 1 {
			avail = 1
		}
		lines := []string{s.fit(head)}
		height := n
		windowed := n > avail
		if windowed {
			height = avail - 2
			if height < 1 {
				height = 1
			}
		}
		pageSize = height
		focus := cur
		if focus > n-1 {
			focus = n - 1
		}
		if focus < start {
			start = focus
		}
		if focus >= start+height {
			start = focus - height + 1
		}
		if start > n-height {
			start = n - height
		}
		if start < 0 {
			start = 0
		}
		if windowed {
			lines = append(lines, more(tty.Symbol("↑", "^"), start))
		}
		for i := start; i < start+height && i < n; i++ {
			pointer := " "
			if i == cur {
				pointer = ">"
			}
			opt := opts[i]
			var row string
			if multi {
				box := "[ ]"
				if selected[i] {
					box = "[x]"
				}
				row = fmt.Sprintf("%s %s %d. %s", pointer, box, i+1, opt.Label)
			} else {
				row = fmt.Sprintf("%s %s", pointer, opt.Label)
			}
			if opt.Description != "" {
				row += " - " + opt.Description
			}
			lines = append(lines, s.fit(row))
		}
		if windowed {
			lines = append(lines, more(tty.Symbol("↓", "v"), n-start-height))
		}
		if multi {
			pointer := " "
			if cur == continueIdx {
				pointer = ">"
			}
			lines = append(lines, s.fit(pointer+" "+continueText))
		}
		if errMsg != "" {
			lines = append(lines, s.fit(s.errorLine(cfg, errMsg)))
		}
		if hint != "" {
			lines = append(lines, s.fit(hint))
		}
		s.draw(lines, len(lines)-1, 0)
	}

	labelOf := func(i int) string { return opts[i].Label }
	summary := func(value string) string { return s.fit(s.header(cfg) + ": " + value) }

	pick := func(i int) (string, bool) {
		value := opts[i].Value
		if cfg.Validate != nil {
			if err := cfg.Validate(value); err != nil {
				errMsg = nonEmpty(err.Error())
				return "", false
			}
		}
		s.finish(summary(labelOf(i)))
		return value, true
	}
	finishMulti := func() (string, bool) {
		value := formatSelected(opts, selected)
		if cfg.Validate != nil {
			if err := cfg.Validate(value); err != nil {
				errMsg = nonEmpty(err.Error())
				return "", false
			}
		} else if value == "" && countSelected(selected) == 0 {
			errMsg = "Please select at least one option"
			return "", false
		}
		var labels []string
		for i := range opts {
			if selected[i] {
				labels = append(labels, labelOf(i))
			}
		}
		s.finish(summary(strings.Join(labels, ", ")))
		return value, true
	}
	toggle := func(i int) {
		selected[i] = !selected[i]
		errMsg = ""
	}

	for {
		render()
		ev, err := s.next(ctx)
		if errors.Is(err, errResized) {
			continue
		}
		if err != nil {
			s.finish(s.fit(s.header(cfg)))
			return "", err
		}
		if ev.isPaste {
			continue
		}
		key := ev.key
		if key == KeyCtrlC {
			s.finish(s.fit(s.header(cfg)))
			return "", clix.ErrCancelled
		}
		if cmd, ok := commandFor(key); ok {
			action := dispatchCommand(cfg, clix.PromptKeyState{Command: cmd, Default: cfg.Default}, nil)
			if action.Exit {
				s.finish(s.fit(s.header(cfg)))
				return "", exitError(action)
			}
			if action.Handled {
				continue
			}
			switch key {
			case KeyEscape:
				s.finish(s.fit(s.header(cfg)))
				return "", clix.ErrCancelled
			case KeyEnter:
				if !multi {
					if v, ok := pick(cur); ok {
						return v, nil
					}
				} else if cur == continueIdx {
					if v, ok := finishMulti(); ok {
						return v, nil
					}
				} else {
					toggle(cur)
				}
			}
			continue
		}
		switch {
		case key == KeyUp || key.Rune == 0x10 || key.Rune == 'k':
			if cur > 0 {
				cur--
			} else {
				cur = last
			}
		case key == KeyDown || key.Rune == 0x0e || key.Rune == 'j':
			if cur < last {
				cur++
			} else {
				cur = 0
			}
		case key == KeyPageUp:
			if cur -= pageSize; cur < 0 {
				cur = 0
			}
		case key == KeyPageDown:
			if cur += pageSize; cur > n-1 {
				cur = n - 1
			}
		case key == KeyHome:
			cur = 0
		case key == KeyEnd:
			cur = last
		case key == KeySpace && multi:
			if cur == continueIdx {
				if v, ok := finishMulti(); ok {
					return v, nil
				}
			} else {
				toggle(cur)
			}
		case key.Rune >= '1' && key.Rune <= '9' && key.Code == byte(key.Rune):
			i := int(key.Rune - '1')
			if i >= n {
				continue
			}
			cur = i
			if multi {
				toggle(i)
			} else if v, ok := pick(i); ok {
				return v, nil
			}
		}
	}
}

func nonEmpty(msg string) string {
	if msg == "" {
		return "invalid answer"
	}
	return msg
}

func countSelected(selected map[int]bool) int {
	n := 0
	for _, on := range selected {
		if on {
			n++
		}
	}
	return n
}
