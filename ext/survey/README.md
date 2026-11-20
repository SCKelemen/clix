# Survey Extension

The Survey extension enables chaining prompts together in a depth-first traversal pattern, allowing dynamic question flows where answers can trigger additional questions.

## Features

- **Depth-first traversal**: Questions are processed from the top of the stack, ensuring nested questions complete before returning to parent questions
- **Dynamic question flow**: Handlers can add new questions based on answers, creating conditional survey branches
- **Functional style**: Chain prompts together using handlers that receive answers and can add more questions
- **Recursive structures**: Support for loops and recursive question patterns (e.g., "add another child?" → add child → "add another child?")
- **Every prompt type the prompter supports**: text input and confirm prompts with `clix.TextPrompter`; text input, confirm, select, and multi-select prompts with `prompt.TerminalPrompter` (from `ext/prompt`)

## Usage

```go
import (
    "github.com/SCKelemen/clix/v2"
    "github.com/SCKelemen/clix/v2/ext/survey"
)

// Create a survey
s := survey.New(ctx, app.Prompter)

// Add questions - handlers can dynamically add more questions
s.Ask(clix.PromptRequest{
    Label: "Do you want to add a child?",
    Confirm: true,
}, func(answer string, s *survey.Survey) {
    if answer == "y" {
        // Add nested question
        s.Ask(clix.PromptRequest{
            Label: "Child's name",
        }, func(name string, s *survey.Survey) {
            // Process name, maybe add more questions
            s.Ask(clix.PromptRequest{
                Label: "Do you want to add another child?",
                Confirm: true,
            }, func(answer2 string, s *survey.Survey) {
                // Recursive: if yes, this could trigger another "add child" flow
            })
        })
    }
})

// Run the survey
if err := s.Run(); err != nil {
    return err
}

// Access collected answers
for _, answer := range s.Answers() {
    fmt.Println(answer)
}
```

## Declarative Question Trees

Prefer a declarative, struct-based API? Define questions up-front using `survey.Question` and wire them together with branches. This mirrors how commands and flags are defined elsewhere in clix.

```go
questions := []survey.Question{
    {
        ID: "start",
        Request: clix.PromptRequest{
            Label:   "Do you have children?",
            Confirm: true,
        },
        Branches: map[string]survey.Branch{
            "y": survey.PushQuestion("how-many"),
            "n": survey.End(),
        },
    },
    {
        ID: "how-many",
        Request: clix.PromptRequest{
            Label: "How many?",
        },
        Branches: map[string]survey.Branch{
            "": survey.End(), // Always end after this question
        },
    },
}

s := survey.NewFromQuestions(ctx, app.Prompter, questions, "start",
    survey.WithUndoStack(),
    survey.WithEndCardText("Survey complete. Export responses?"),
)
if err := s.Run(); err != nil {
    return err
}
```

A question loops by pushing an earlier question again:

```go
questions := []survey.Question{
    {
        ID: "add-child",
        Request: clix.PromptRequest{
            Label: "Do you want to add a child?",
            Options: []clix.SelectOption{
                {Label: "Yes", Value: "yes"},
                {Label: "No", Value: "no"},
            },
        },
        Branches: map[string]survey.Branch{
            "yes": survey.PushQuestion("child-name"),
            "no":  survey.End(),
        },
    },
    {
        ID:      "child-name",
        Request: clix.PromptRequest{Label: "Child's name"},
        Branches: map[string]survey.Branch{
            "": survey.PushQuestion("add-another"), // Default: always continue
        },
    },
    {
        ID: "add-another",
        Request: clix.PromptRequest{
            Label:   "Add another child?",
            Confirm: true,
        },
        Branches: map[string]survey.Branch{
            "y": survey.PushQuestion("child-name"), // Loop back
            "n": survey.End(),
        },
    },
}

s := survey.NewFromQuestions(ctx, app.Prompter, questions, "add-child")
if err := s.Run(); err != nil {
    return err
}
```

Questions can also be added one by one, then started:

```go
s := survey.New(ctx, app.Prompter)
s.AddQuestion(survey.Question{
    ID:       "name",
    Request:  clix.PromptRequest{Label: "Name"},
    Branches: map[string]survey.Branch{"": survey.End()},
})
s.Start("name")
if err := s.Run(); err != nil {
    return err
}
```

The older fluent API does the same: `s.Question("name", clix.PromptRequest{Label: "Name"}).Then("choose")` asks `choose` next, and `.End()` ends the survey after a question.

You can still mix in dynamic questions by calling `Survey.Ask` within branch handlers—`NewFromQuestions` simply seeds the initial tree.

## Options

- **`survey.WithUndoStack()`** lets users return to the previous question and change their answer. In raw mode (`prompt.TerminalPrompter` from `ext/prompt` on an interactive terminal) they press Esc or F12. In line mode (`clix.TextPrompter`, or input that is not a terminal) they type `back` at a text or confirm question. Going back restores the questions exactly as they were before that question was asked.
- **`survey.WithEndCard()`** shows a formatted summary of all answers (with styling support; secret answers are masked) and asks for confirmation. If undo is enabled, users can answer `no` (or press Esc or F12 with `TerminalPrompter`) to go back and edit answers.
- **`survey.WithEndCardText(text)`** replaces the end card's confirmation question.
- **`survey.WithEndCardTheme(theme)`** styles the end card.

## Depth-First Traversal

Questions are processed depth-first, meaning when a question's handler adds new questions, those new questions are immediately processed before returning to process other questions at the same level.

Example flow:
1. Ask "Do you want to add a child?" → Yes
2. Handler adds "Child's name" question
3. **Immediately asks "Child's name"** (depth-first: new question processed first)
4. Handler adds "Add another child?" question
5. **Immediately asks "Add another child?"** (continues depth-first)
6. If yes, the cycle repeats; if no, completes and returns

This prevents "topic jumping" - you complete a branch fully before moving on.

The questions wait on a stack, which is last-in, first-out: questions added together with `Ask` are asked in the reverse of the order they were added. Add them in reverse to ask them in reading order, as the examples below do.

## Examples

### Simple Linear Survey

```go
s := survey.New(ctx, app.Prompter)
// Last in, first out: "First name" is asked first.
s.Ask(clix.PromptRequest{Label: "Last name"}, nil)
s.Ask(clix.PromptRequest{Label: "First name"}, nil)
s.Run()
```

### Conditional Branching

```go
s := survey.New(ctx, app.Prompter)
s.Ask(clix.PromptRequest{
    Label: "Do you have children?",
    Confirm: true,
}, func(answer string, s *survey.Survey) {
    if answer == "y" {
        // Asks "How many?", then "Oldest child's name?".
        s.Ask(clix.PromptRequest{Label: "Oldest child's name?"}, nil)
        s.Ask(clix.PromptRequest{Label: "How many?"}, nil)
    }
})
s.Run()
```

### Recursive Pattern (Add Children Loop)

```go
var children []string

// Declare the variable first so the closure can call itself.
var addChild func(s *survey.Survey)
addChild = func(s *survey.Survey) {
    s.Ask(clix.PromptRequest{Label: "Child's name"}, func(name string, s *survey.Survey) {
        children = append(children, name)
        s.Ask(clix.PromptRequest{
            Label: "Add another child?",
            Confirm: true,
        }, func(answer string, s *survey.Survey) {
            if answer == "y" {
                addChild(s) // Recursive: add another child
            }
        })
    })
}

s := survey.New(ctx, app.Prompter)
addChild(s)
if err := s.Run(); err != nil {
    return err
}
fmt.Println("Children:", children)
```

## API Reference

### `survey.New(ctx context.Context, prompter clix.Prompter, options ...SurveyOption) *Survey`

Creates a new survey instance.

### `survey.NewFromQuestions(ctx context.Context, prompter clix.Prompter, questions []Question, startID string, options ...SurveyOption) *Survey`

Creates a survey from `Question` struct literals and starts it at `startID`.

### `Survey.AddQuestion(q Question)`, `Survey.AddQuestions(questions []Question)`, `Survey.Start(questionID string)`

Register static questions and choose the first one to ask.

### `Survey.Ask(request clix.PromptRequest, handler func(answer string, survey *Survey))`

Adds a question to the top of the survey's stack, so it is asked before the questions already waiting. The handler is called with the answer and can add more questions to the survey.

### `Survey.Run() error`

Executes all questions in the survey using depth-first traversal.

### Branches: `survey.PushQuestion(id)`, `survey.End()`, `survey.Handler(fn)`

Decide what happens after a question is answered. Branches are keyed by answer; the `""` key applies when no other key matches.

### Options: `survey.WithUndoStack()`, `survey.WithEndCard()`, `survey.WithEndCardText(text)`, `survey.WithEndCardTheme(theme)`

Enable going back to previous questions, a summary with a final confirmation, and the end card's text and styling.

### `Survey.Answers() []string`

Returns all collected answers in the order they were answered.

### `Survey.Clear()`

Removes all remaining questions from the survey stack.

