# Validation Extension

The `validation` package provides common validators for use with `clix` prompts and flags.

## Usage

```go
import (
    "clix"
    "clix/ext/validation"
)

// Use with prompts
prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Email",
    Validate: validation.Email,
})

// Use with flags (combined with other validators)
app.Flags.StringVar(&clix.StringVarOptions{
    Name:    "api-url",
    Usage:   "API endpoint URL",
    Value:   &url,
    Validate: validation.URL,
})

// Combine validators
validator := validation.All(
    validation.NotEmpty,
    validation.MinLength(8),
    validation.MaxLength(64),
)

prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Password",
    Validate: validator,
})
```

## Available Validators

### Email & URL
- `Email(value string) error` - Validates RFC 5322 email addresses
- `URL(value string) error` - Validates URLs (must include scheme and host)

### Network
- `CIDR(value string) error` - Validates CIDR notation (e.g., "192.168.1.0/24")
- `IPv4(value string) error` - Validates IPv4 addresses
- `IPv6(value string) error` - Validates IPv6 addresses
- `IP(value string) error` - Validates IPv4 or IPv6 addresses
- `Port(value string) error` - Validates TCP/UDP port numbers (1-65535)
- `Hostname(value string) error` - Validates hostnames according to RFC 1123

### Phone Numbers
- `E164(value string) error` - Validates E.164 phone numbers (e.g., "+1234567890")

### Identifiers
- `UUID(value string) error` - Validates UUID strings (e.g., "550e8400-e29b-41d4-a716-446655440000")

### Numeric
- `Integer(value string) error` - Validates that a string can be parsed as an integer (int)
- `Int64(value string) error` - Validates that a string can be parsed as an int64
- `Float64(value string) error` - Validates that a string can be parsed as a float64
- `IntRange(min, max int) Validator` - Validates integer within range [min, max]
- `FloatRange(min, max float64) Validator` - Validates float64 within range [min, max]

### String Constraints
- `NotEmpty(value string) error` - Ensures string is not empty (after trimming)
- `MinLength(min int) Validator` - Ensures minimum string length
- `MaxLength(max int) Validator` - Ensures maximum string length
- `Length(exact int) Validator` - Ensures exact string length
- `Regex(pattern string) Validator` - Validates against a regular expression

### Combinators
- `All(validators ...Validator) Validator` - All validators must pass
- `Any(validators ...Validator) Validator` - At least one validator must pass

