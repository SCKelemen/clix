# Survey Extension

The Survey extension enables chaining prompts together in a depth-first traversal pattern, allowing dynamic question flows where answers can trigger additional questions.

## Features

- **Depth-first traversal**: Questions are processed from the top of the stack, ensuring nested questions complete before returning to parent questions
- **Dynamic question flow**: Handlers can add new questions based on answers, creating conditional survey branches
- **Functional style**: Chain prompts together using handlers that receive answers and can add more questions
- **Recursive structures**: Support for loops and recursive question patterns (e.g., "add another child?" → add child → "add another child?")

## Usage

```go
import (
    "clix"
    "clix/ext/survey"
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
answers := s.Answers()
```

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

## Examples

### Simple Linear Survey

```go
s := survey.New(ctx, app.Prompter)
s.Ask(clix.PromptRequest{Label: "First name"}, nil)
s.Ask(clix.PromptRequest{Label: "Last name"}, nil)
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
        s.Ask(clix.PromptRequest{Label: "How many?"}, nil)
        s.Ask(clix.PromptRequest{Label: "Oldest child's name?"}, nil)
    }
})
s.Run()
```

### Recursive Pattern (Add Children Loop)

```go
var children []string

addChild := func(s *survey.Survey) {
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
s.Run()
```

## API Reference

### `survey.New(ctx context.Context, prompter clix.Prompter) *Survey`

Creates a new survey instance.

### `Survey.Ask(request clix.PromptRequest, handler func(answer string, survey *Survey))`

Adds a question to the survey. The handler is called with the answer and can add more questions to the survey.

### `Survey.Run() error`

Executes all questions in the survey using depth-first traversal.

### `Survey.Answers() []string`

Returns all collected answers in the order they were answered.

### `Survey.Clear()`

Removes all remaining questions from the survey stack.

