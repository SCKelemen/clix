# 9. Surveys

The Survey extension enables chaining multiple prompts together in a depth-first traversal pattern, allowing dynamic question flows where answers can trigger additional questions.

## Basic Survey

Create a survey and add questions:

```go
package main

import (
    "context"
    "fmt"
    "os"
    
    "clix"
    "clix/ext/prompt"
    "clix/ext/survey"
)

func main() {
    app := clix.NewApp("demo")
    app.Out = os.Stdout
    app.In = os.Stdin
    
    // Add extensions
    app.AddExtension(prompt.Extension{})
    app.AddExtension(survey.Extension{})
    
    if err := app.ApplyExtensions(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    
    cmd := clix.NewCommand("survey")
    cmd.Run = func(ctx *clix.Context) error {
        s := survey.New(context.Background(), ctx.App.Prompter)
        
        // Add questions dynamically
        s.Ask(clix.PromptRequest{
            Label: "What is your name?",
            Theme: ctx.App.DefaultTheme,
        }, func(answer string, s *survey.Survey) {
            // Handler receives answer and can add more questions
            fmt.Fprintf(ctx.App.Out, "Hello, %s!\n", answer)
        })
        
        // Run the survey
        return s.Run()
    }
    
    app.Root = cmd
    
    if err := app.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

## Depth-First Traversal

Questions are processed depth-first: when a question's handler adds new questions, those new questions are processed immediately before returning to the parent.

```go
s.Ask(clix.PromptRequest{
    Label: "Do you want to add a child?",
    Confirm: true,
}, func(answer string, s *survey.Survey) {
    if answer == "y" {
        // Add nested question - it's processed immediately (depth-first)
        s.Ask(clix.PromptRequest{
            Label: "Child's name",
        }, func(name string, s *survey.Survey) {
            fmt.Printf("Child: %s\n", name)
            
            // Can add even more questions here
            s.Ask(clix.PromptRequest{
                Label: "Add another child?",
                Confirm: true,
            }, func(answer2 string, s *survey.Survey) {
                // Recursive: if yes, could trigger another "add child" flow
            })
        })
    }
})
```

**Flow:**
1. Ask "Do you want to add a child?" → Yes
2. **Immediately** ask "Child's name" (depth-first)
3. After name is answered, ask "Add another child?"
4. Return to process any other questions at the same level

## Static Survey Definition

For surveys with known structure, define questions statically:

```go
questions := []survey.Question{
    {
        ID: "name",
        Request: clix.PromptRequest{
            Label: "What is your name?",
            Theme: ctx.App.DefaultTheme,
        },
        Branches: map[string]survey.Branch{
            "": survey.PushQuestion("email"),  // Always ask email next
        },
    },
    {
        ID: "email",
        Request: clix.PromptRequest{
            Label: "What is your email?",
            Theme: ctx.App.DefaultTheme,
        },
        Branches: map[string]survey.Branch{
            "": survey.End(),  // End survey
        },
    },
}

s := survey.NewFromQuestions(context.Background(), ctx.App.Prompter, questions, "name")
return s.Run()
```

## Conditional Branches

Use branches to create conditional flows:

```go
questions := []survey.Question{
    {
        ID: "has-account",
        Request: clix.PromptRequest{
            Label:   "Do you have an account?",
            Confirm: true,
            Theme:    ctx.App.DefaultTheme,
        },
        Branches: map[string]survey.Branch{
            "y": survey.PushQuestion("login"),     // If yes, ask login
            "n": survey.PushQuestion("signup"),   // If no, ask signup
        },
    },
    {
        ID: "login",
        Request: clix.PromptRequest{
            Label: "Username",
            Theme: ctx.App.DefaultTheme,
        },
        Branches: map[string]survey.Branch{
            "": survey.End(),
        },
    },
    {
        ID: "signup",
        Request: clix.PromptRequest{
            Label: "Choose username",
            Theme: ctx.App.DefaultTheme,
        },
        Branches: map[string]survey.Branch{
            "": survey.End(),
        },
    },
}
```

## Undo/Back Functionality

Enable undo stack to allow users to go back:

```go
s := survey.NewFromQuestions(ctx, prompter, questions, "start",
    survey.WithUndoStack(),  // Enable undo
)

// Users can press Escape or F12 to go back to previous question
```

## End Card

Show a summary of all answers at the end:

```go
s := survey.NewFromQuestions(ctx, prompter, questions, "start",
    survey.WithEndCard(),  // Show summary at end
    survey.WithUndoStack(),  // Allow going back from summary
)
```

The end card displays:
- All questions asked
- All answers provided
- Option to confirm or go back (if undo enabled)

## Survey Builder API

For backward compatibility, use the builder pattern:

```go
s := survey.New(ctx, prompter)

s.Question("name", clix.PromptRequest{
    Label: "Name",
    Theme: ctx.App.DefaultTheme,
}).Then("email")  // Next question

s.Question("email", clix.PromptRequest{
    Label: "Email",
    Theme: ctx.App.DefaultTheme,
}).End()  // End survey

s.Start("name")
return s.Run()
```

## Accessing Answers

Get all collected answers:

```go
answers := s.Answers()
// Returns []string of all answers in order
```

## Validation in Surveys

Validation works with surveys:

```go
import "clix/ext/validation"

questions := []survey.Question{
    {
        ID: "email",
        Request: clix.PromptRequest{
            Label:   "Email",
            Validate: validation.Email(),
            Theme:    ctx.App.DefaultTheme,
        },
    },
}
```

## Next Steps

Now that you understand surveys, learn about the [Extension System](10_extensions.md) to understand how all these features work together.

