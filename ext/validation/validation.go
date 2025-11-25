package validation

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

// Validator is a function that validates a string value and returns an error if invalid.
// Validators can be used with prompts, arguments, and flags.
//
// Example:
//
//	import "github.com/SCKelemen/clix/ext/validation"
//
//	// Use with prompts
//	result, err := prompter.Prompt(ctx, clix.PromptRequest{
//		Label:   "Email",
//		Validate: validation.Email,
//	})
//
//	// Use with arguments
//	cmd.Arguments = []*clix.Argument{
//		{
//			Name:     "email",
//			Required: true,
//			Validate: validation.Email,
//		},
//	}
//
//	// Combine validators
//	validate := validation.All(
//		validation.NotEmpty,
//		validation.MinLength(8),
//		validation.Regex(`^[a-zA-Z0-9]+$`),
//	)
type Validator func(string) error

// Email validates an RFC 5322 compliant email address.
func Email(value string) error {
	if value == "" {
		return errors.New("email cannot be empty")
	}

	// RFC 5322 email regex (simplified but covers most cases)
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(value) {
		return errors.New("invalid email address")
	}

	// Additional check: must have a domain
	parts := strings.Split(value, "@")
	if len(parts) != 2 || parts[1] == "" {
		return errors.New("invalid email address")
	}

	return nil
}

// URL validates a URL string.
func URL(value string) error {
	if value == "" {
		return errors.New("URL cannot be empty")
	}

	u, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme == "" {
		return errors.New("URL must include a scheme (e.g., http://, https://)")
	}

	if u.Host == "" {
		return errors.New("URL must include a host")
	}

	return nil
}

// CIDR validates a CIDR notation IP address range.
func CIDR(value string) error {
	if value == "" {
		return errors.New("CIDR cannot be empty")
	}

	_, _, err := net.ParseCIDR(value)
	if err != nil {
		return fmt.Errorf("invalid CIDR: %w", err)
	}

	return nil
}

// IPv4 validates an IPv4 address.
func IPv4(value string) error {
	if value == "" {
		return errors.New("IPv4 address cannot be empty")
	}

	ip := net.ParseIP(value)
	if ip == nil {
		return errors.New("invalid IPv4 address")
	}

	if ip.To4() == nil {
		return errors.New("not an IPv4 address")
	}

	return nil
}

// IPv6 validates an IPv6 address.
func IPv6(value string) error {
	if value == "" {
		return errors.New("IPv6 address cannot be empty")
	}

	ip := net.ParseIP(value)
	if ip == nil {
		return errors.New("invalid IPv6 address")
	}

	if ip.To16() == nil || ip.To4() != nil {
		return errors.New("not an IPv6 address")
	}

	return nil
}

// IP validates an IPv4 or IPv6 address.
func IP(value string) error {
	if value == "" {
		return errors.New("IP address cannot be empty")
	}

	ip := net.ParseIP(value)
	if ip == nil {
		return errors.New("invalid IP address")
	}

	return nil
}

// E164 validates an E.164 phone number (e.g., +1234567890).
func E164(value string) error {
	if value == "" {
		return errors.New("phone number cannot be empty")
	}

	// E.164 format: + followed by 1-15 digits
	e164Regex := regexp.MustCompile(`^\+[1-9]\d{1,14}$`)
	if !e164Regex.MatchString(value) {
		return errors.New("invalid E.164 phone number (must start with + followed by country code and number)")
	}

	return nil
}

// NotEmpty validates that a string is not empty (after trimming whitespace).
func NotEmpty(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("cannot be empty")
	}
	return nil
}

// MinLength validates that a string has at least the specified minimum length.
func MinLength(min int) Validator {
	return func(value string) error {
		if len(value) < min {
			return fmt.Errorf("must be at least %d characters", min)
		}
		return nil
	}
}

// MaxLength validates that a string has at most the specified maximum length.
func MaxLength(max int) Validator {
	return func(value string) error {
		if len(value) > max {
			return fmt.Errorf("must be at most %d characters", max)
		}
		return nil
	}
}

// Length validates that a string has exactly the specified length.
func Length(exact int) Validator {
	return func(value string) error {
		if len(value) != exact {
			return fmt.Errorf("must be exactly %d characters", exact)
		}
		return nil
	}
}

// Regex validates that a string matches the specified regular expression.
func Regex(pattern string) Validator {
	re := regexp.MustCompile(pattern)
	return func(value string) error {
		if !re.MatchString(value) {
			return fmt.Errorf("must match pattern: %s", pattern)
		}
		return nil
	}
}

// All combines multiple validators, requiring all to pass.
func All(validators ...Validator) Validator {
	return func(value string) error {
		for _, validator := range validators {
			if err := validator(value); err != nil {
				return err
			}
		}
		return nil
	}
}

// Any combines multiple validators, requiring at least one to pass.
func Any(validators ...Validator) Validator {
	return func(value string) error {
		var lastErr error
		for _, validator := range validators {
			if err := validator(value); err == nil {
				return nil
			} else {
				lastErr = err
			}
		}
		if lastErr == nil {
			return errors.New("validation failed")
		}
		return lastErr
	}
}
