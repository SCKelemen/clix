# Comparison: clix Survey Extension vs charmbracelet/huh

## Overview

Both `clix/ext/survey` and `charmbracelet/huh` provide form-building capabilities for terminal applications, but they take different approaches to API design, architecture, and use cases.

## Key Differences

### 1. **Architecture & Dependencies**

| Feature | clix Survey | charmbracelet/huh |
|---------|-------------|-------------------|
| **Dependencies** | No event loop, lightweight | Requires Bubble Tea (event loop) |
| **Model** | Prompt-and-return, terminating | Long-running TUI application |
| **Integration** | Works with any `clix.Prompter` | Requires Bubble Tea integration |
| **Terminal Control** | Delegates to prompter (TextPrompter or TerminalPrompter) | Full terminal control via Bubble Tea |

**clix Survey**: Lightweight, no event loop. Each prompt returns a value and exits. Works with both interactive (TerminalPrompter) and non-interactive (TextPrompter) modes.

**huh**: Built on Bubble Tea, requires a full event loop. Forms are long-running TUI applications that take over the terminal.

### 2. **API Style**

#### clix Survey - Dual API Style

**Functional/Handler-based:**
```go
s := survey.New(ctx, app.Prompter)
s.Ask(clix.PromptRequest{
    Label: "Do you have children?",
    Confirm: true,
}, func(answer string, s *survey.Survey) {
    if answer == "y" {
        s.Ask(clix.PromptRequest{Label: "How many?"}, nil)
    }
})
s.Run()
```

**Declarative/Struct-based:**
```go
questions := []survey.Question{
    {
        ID: "has-children",
        Request: clix.PromptRequest{
            Label: "Do you have children?",
            Confirm: true,
        },
        Branches: map[string]survey.Branch{
            "y": survey.PushQuestion("how-many"),
            "n": survey.End(),
        },
    },
    {
        ID: "how-many",
        Request: clix.PromptRequest{Label: "How many?"},
        Branches: map[string]survey.Branch{"": survey.End()},
    },
}
s := survey.NewFromQuestions(ctx, app.Prompter, questions, "has-children")
s.Run()
```

#### huh - Declarative Form API

```go
var (
    hasChildren bool
    childCount  int
)

form := huh.NewForm(
    huh.NewGroup(
        huh.NewConfirm().
            Title("Do you have children?").
            Value(&hasChildren),
    ),
    huh.NewGroup(
        huh.NewInput().
            Title("How many?").
            Value(&childCount).
            Validate(func(i int) error {
                if !hasChildren {
                    return nil // Skip validation if no children
                }
                if i < 0 {
                    return errors.New("must be non-negative")
                }
                return nil
            }),
    ).WithHideFunc(func() bool {
        return !hasChildren // Hide if no children
    }),
)

err := form.Run()
```

**Key Difference**: 
- **clix Survey**: Supports both functional (handler-based) and declarative (struct-based) APIs. Questions are processed depth-first via a stack.
- **huh**: Purely declarative, form-based API. Uses Bubble Tea's update loop for reactivity.

### 3. **Question Flow Control**

#### clix Survey - Depth-First Stack-Based

```go
// Depth-first: nested questions complete before returning to parent
s.Ask(clix.PromptRequest{Label: "Add child?"}, func(answer string, s *survey.Survey) {
    if answer == "y" {
        s.Ask(clix.PromptRequest{Label: "Child name"}, func(name string, s *survey.Survey) {
            // This completes fully before any other questions
            s.Ask(clix.PromptRequest{Label: "Add another?"}, nil)
        })
    }
})
```

**Flow**: Questions are processed from a stack (LIFO). When a handler adds questions, they're immediately processed (depth-first).

#### huh - Reactive Form Updates

```go
form := huh.NewForm(
    huh.NewGroup(
        huh.NewConfirm().
            Title("Add child?").
            Value(&addChild),
    ),
    huh.NewGroup(
        huh.NewInput().
            Title("Child name").
            Value(&childName).
            WithHideFunc(func() bool {
                return !addChild // Reactively hide/show based on other fields
            }),
    ),
)
```

**Flow**: Form fields can reactively show/hide and validate based on other fields' values. Uses Bubble Tea's update loop for real-time reactivity.

**Key Difference**:
- **clix Survey**: Explicit branching via handlers or branch maps. Depth-first traversal ensures nested flows complete fully.
- **huh**: Reactive forms where fields can hide/show and validate based on other fields. All fields are part of a single form model.

### 4. **Field Types & Validation**

#### clix Survey

Uses `clix.PromptRequest` which supports:
- Text input
- Confirm (yes/no)
- Select (single choice)
- MultiSelect (multiple choices)
- Password (masked input)
- All prompt types from `ext/prompt` extension

Validation via `ext/validation`:
```go
s.Ask(clix.PromptRequest{
    Label: "Email",
    Validate: validation.Email,
}, nil)
```

#### huh

Built-in field types:
- `Input` (text)
- `Text` (multi-line)
- `Confirm` (yes/no)
- `Select` (single choice)
- `MultiSelect` (multiple choices)
- `FilePicker` (file selection)

Validation:
```go
huh.NewInput().
    Title("Email").
    Value(&email).
    Validate(func(s string) error {
        if !strings.Contains(s, "@") {
            return errors.New("invalid email")
        }
        return nil
    })
```

**Key Difference**:
- **clix Survey**: Uses existing `clix.PromptRequest` API, validation via `ext/validation` package.
- **huh**: Built-in field types with inline validation functions.

### 5. **Advanced Features**

#### clix Survey

- **Undo/Back Stack**: `survey.WithUndoStack()` enables going back to previous questions
- **End Card**: `survey.WithEndCard()` shows a summary and confirmation at the end
- **Dynamic Questions**: Handlers can add questions on-the-fly
- **Recursive Patterns**: Support for loops (e.g., "add another child?")
- **Dual Prompter Support**: Works with TextPrompter (non-interactive) and TerminalPrompter (interactive)

```go
s := survey.NewFromQuestions(ctx, app.Prompter, questions, "start",
    survey.WithUndoStack(),    // Enable back navigation
    survey.WithEndCard(),       // Show summary at end
)
```

#### huh

- **Reactive Hiding**: Fields can hide/show based on other fields
- **Dynamic Options**: Select options can be computed from other fields
- **Theming**: Integrated with Lip Gloss for styling
- **Accessible Mode**: Screen reader support
- **Form State**: All fields in a single form model

```go
huh.NewSelect().
    Title("Choose").
    Options(huh.NewOptions("opt1", "opt2")...).
    WithHideFunc(func() bool {
        return !someCondition
    })
```

**Key Difference**:
- **clix Survey**: Focus on question flow control (undo, end cards, dynamic branching)
- **huh**: Focus on reactive form behavior (hide/show, dynamic options, form-wide state)

### 6. **Styling**

#### clix Survey

Uses `clix.PromptTheme` (compatible with lipgloss):
```go
s.Ask(clix.PromptRequest{
    Label: "Name",
    Theme: clix.PromptTheme{
        LabelStyle: lipgloss.NewStyle().Bold(true),
        PrefixStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("63")),
    },
}, nil)
```

#### huh

Integrated with Lip Gloss:
```go
theme := huh.ThemeCharm()
theme.Focused.Title = lipgloss.NewStyle().Bold(true)
form := huh.NewForm(...).WithTheme(theme)
```

**Key Difference**: Both use lipgloss, but huh has more built-in theming support.

### 7. **Use Cases**

#### clix Survey - Best For

- **CLI command flows**: Multi-step configuration, setup wizards
- **Conditional branching**: Complex decision trees
- **Recursive patterns**: "Add another item?" loops
- **Non-interactive fallback**: Works with TextPrompter for scripts
- **Integration with clix commands**: Natural fit for `clix.Command` handlers

```go
cmd.Run = func(ctx *clix.Context) error {
    s := survey.NewFromQuestions(ctx, ctx.App.Prompter, questions, "start")
    return s.Run()
}
```

#### huh - Best For

- **Standalone form applications**: Full-screen form UIs
- **Reactive forms**: Fields that depend on each other
- **Bubble Tea applications**: Already using Bubble Tea for other UI
- **Complex validation**: Cross-field validation
- **Accessible forms**: Screen reader support

### 8. **Code Size & Complexity**

| Metric | clix Survey | huh |
|--------|-------------|-----|
| **Lines of Code** | ~900 lines | Larger (includes Bubble Tea integration) |
| **Dependencies** | Only clix core + ext/prompt | Bubble Tea, Lip Gloss |
| **Learning Curve** | Low (familiar if using clix) | Medium (requires Bubble Tea knowledge) |
| **Terminal Control** | Delegated to prompter | Full control via Bubble Tea |

## Summary

### When to Use clix Survey

✅ You're building a CLI tool with `clix`  
✅ You need conditional branching and recursive patterns  
✅ You want lightweight, prompt-and-return behavior  
✅ You need non-interactive fallback (TextPrompter)  
✅ You want undo/back navigation  
✅ You prefer explicit control flow over reactive forms  

### When to Use huh

✅ You're building a standalone form application  
✅ You need reactive forms (fields hide/show based on others)  
✅ You're already using Bubble Tea  
✅ You want built-in accessible mode  
✅ You prefer declarative form definitions  
✅ You need complex cross-field validation  

## Hybrid Approach

You can use both! `clix` survey for CLI command flows, and `huh` for standalone form applications. They serve different purposes:

- **clix Survey**: Part of a larger CLI application, terminating commands
- **huh**: Standalone form applications, long-running TUI

## Conclusion

**clix Survey** is designed for CLI applications that need multi-step prompts as part of command execution. It's lightweight, integrates seamlessly with `clix`, and supports both interactive and non-interactive modes.

**huh** is designed for standalone form applications with reactive behavior. It requires Bubble Tea but provides more built-in form features like reactive hiding and accessible mode.

Both are excellent tools for their respective use cases. Choose based on whether you're building a CLI tool (clix Survey) or a standalone form application (huh).

