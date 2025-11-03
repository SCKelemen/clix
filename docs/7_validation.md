# 7. Validation

Validation ensures users provide correct input. CLIX supports validation for both arguments and prompts, with built-in validators available via the Validation extension.

## Basic Validation

Add validation functions to arguments:

```go
package main

import (
    "errors"
    "fmt"
    "os"
    "strconv"
    
    "clix"
)

func main() {
    app := clix.NewApp("demo")
    app.Out = os.Stdout
    app.In = os.Stdin
    
    cmd := clix.NewCommand("age")
    cmd.Arguments = []*clix.Argument{
        {
            Name:     "age",
            Prompt:   "Your age",
            Required: true,
            Validate: func(value string) error {
                age, err := strconv.Atoi(value)
                if err != nil {
                    return errors.New("age must be a number")
                }
                if age < 0 {
                    return errors.New("age cannot be negative")
                }
                if age > 150 {
                    return errors.New("age cannot be greater than 150")
                }
                return nil
            },
        },
    }
    
    cmd.Run = func(ctx *clix.Context) error {
        fmt.Fprintf(ctx.App.Out, "Your age is %s\n", ctx.Args[0])
        return nil
    }
    
    app.Root = cmd
    
    if err := app.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

### Example Interaction

```bash
$ go run main.go age 25
Your age is 25

$ go run main.go age abc
Error: age must be a number

$ go run main.go age -5
Error: age cannot be negative
```

## Prompt Validation

Validation also works with prompts:

```go
email, err := ctx.App.Prompter.Prompt(context.Background(), clix.PromptRequest{
    Label: "Email address",
    Validate: func(value string) error {
        if !strings.Contains(value, "@") {
            return errors.New("email must contain @")
        }
        return nil
    },
    Theme: ctx.App.DefaultTheme,
})
```

The prompt will re-prompt if validation fails:

```bash
$ go run main.go
Email address: invalid
Error: email must contain @
Email address: user@example.com
# Continues...
```

## Validation Extension

The Validation extension provides common validators. Add it to your app:

```go
import "clix/ext/validation"

app.AddExtension(validation.Extension{})
```

Now you can use built-in validators:

```go
import "clix/ext/validation"

cmd.Arguments = []*clix.Argument{
    {
        Name:     "email",
        Prompt:   "Email address",
        Required: true,
        Validate: validation.Email(),  // Built-in email validator
    },
}
```

### Available Validators

**Email Validation:**
```go
validation.Email()
// Validates RFC 5322 email format
```

**URL Validation:**
```go
validation.URL()
// Validates HTTP/HTTPS URLs
```

**CIDR Validation:**
```go
validation.CIDR()
// Validates CIDR notation (e.g., "192.168.1.0/24")
```

**IP Address Validation:**
```go
validation.IP()
// Validates IPv4 or IPv6 addresses
```

**E.164 Phone Validation:**
```go
validation.E164()
// Validates E.164 phone numbers (+1234567890)
```

**String Constraints:**
```go
validation.MinLength(5)    // Minimum length
validation.MaxLength(100)   // Maximum length
validation.Length(10, 20)    // Exact range
```

**Regex Validation:**
```go
validation.Regex(regexp.MustCompile(`^[A-Z]+$`))
```

## Combining Validators

Use `validation.All` to combine multiple validators:

```go
import "clix/ext/validation"

cmd.Arguments = []*clix.Argument{
    {
        Name:     "password",
        Prompt:   "Password",
        Required: true,
        Validate: validation.All(
            validation.MinLength(8),
            validation.Regex(regexp.MustCompile(`[A-Z]`)),  // At least one uppercase
            validation.Regex(regexp.MustCompile(`[a-z]`)),  // At least one lowercase
            validation.Regex(regexp.MustCompile(`[0-9]`)),  // At least one digit
        ),
    },
}
```

Use `validation.Any` to allow any of multiple validators to pass:

```go
Validate: validation.Any(
    validation.IP(),
    validation.CIDR(),
),  // Accepts either IP or CIDR
```

## Custom Error Messages

Validators return errors that are displayed to users:

```go
Validate: func(value string) error {
    if len(value) < 5 {
        return fmt.Errorf("password must be at least 5 characters (got %d)", len(value))
    }
    return nil
}
```

## Validation in Prompts

Validation works seamlessly with prompts. Invalid input causes re-prompting:

```go
email, err := ctx.App.Prompter.Prompt(context.Background(), clix.PromptRequest{
    Label: "Email",
    Validate: validation.Email(),
    Theme: ctx.App.DefaultTheme,
})
```

**Example:**
```bash
$ go run main.go
Email: invalid
Error: invalid email format
Email: still-invalid
Error: invalid email format
Email: user@example.com
# Continues...
```

## Next Steps

Now that you understand validation, learn about [Terminal Prompts](8_terminal_prompts.md) for advanced interactive features like select lists and multi-select.

