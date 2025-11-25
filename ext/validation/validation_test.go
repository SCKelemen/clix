package validation

import (
	"strings"
	"testing"
)

func TestEmail(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid email", "user@example.com", false},
		{"valid email with subdomain", "user@mail.example.com", false},
		{"valid email with plus", "user+tag@example.com", false},
		{"valid email with dots", "first.last@example.com", false},
		{"empty email", "", true},
		{"no @ symbol", "userexample.com", true},
		{"no domain", "user@", true},
		{"no local part", "@example.com", true},
		{"no TLD", "user@example", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Email(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Email(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid http URL", "http://example.com", false},
		{"valid https URL", "https://example.com", false},
		{"valid URL with path", "https://example.com/path/to/resource", false},
		{"valid URL with query", "https://example.com?key=value", false},
		{"empty URL", "", true},
		{"no scheme", "example.com", true},
		{"no host", "https://", true},
		{"invalid URL", "not a url", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := URL(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("URL(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestCIDR(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid IPv4 CIDR", "192.168.1.0/24", false},
		{"valid IPv6 CIDR", "2001:db8::/32", false},
		{"empty CIDR", "", true},
		{"invalid CIDR", "192.168.1.0", true},
		{"invalid format", "not a cidr", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CIDR(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("CIDR(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestIPv4(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid IPv4", "192.168.1.1", false},
		{"valid IPv4 loopback", "127.0.0.1", false},
		{"empty IPv4", "", true},
		{"IPv6 address", "2001:db8::1", true},
		{"invalid format", "not an ip", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := IPv4(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("IPv4(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestIPv6(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid IPv6", "2001:db8::1", false},
		{"valid IPv6 loopback", "::1", false},
		{"empty IPv6", "", true},
		{"IPv4 address", "192.168.1.1", true},
		{"invalid format", "not an ip", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := IPv6(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("IPv6(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestIP(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid IPv4", "192.168.1.1", false},
		{"valid IPv6", "2001:db8::1", false},
		{"empty IP", "", true},
		{"invalid format", "not an ip", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := IP(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("IP(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestE164(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid E.164", "+1234567890", false},
		{"valid E.164 long", "+123456789012345", false},
		{"valid E.164 short", "+123456789", false},
		{"empty E.164", "", true},
		{"no plus", "1234567890", true},
		{"starts with 0", "+01234567890", true},
		{"too short", "+1", true},
		{"too long", "+12345678901234567", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := E164(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("E164(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestPort(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid port", "8080", false},
		{"valid min port", "1", false},
		{"valid max port", "65535", false},
		{"empty port", "", true},
		{"port too low", "0", true},
		{"port too high", "65536", true},
		{"negative port", "-1", true},
		{"not a number", "abc", true},
		{"float port", "8080.5", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Port(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Port(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestHostname(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid hostname", "example.com", false},
		{"valid subdomain", "subdomain.example.com", false},
		{"valid localhost", "localhost", false},
		{"valid single label", "example", false},
		{"valid with numbers", "example123.com", false},
		{"valid with hyphens", "my-example.com", false},
		{"empty hostname", "", true},
		{"label too long", strings.Repeat("a", 64) + ".com", true},
		{"total too long", strings.Repeat("a", 254) + ".com", true},
		{"label starts with hyphen", "-example.com", true},
		{"label ends with hyphen", "example-.com", true},
		{"empty label", "example..com", true},
		{"invalid characters", "example@.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Hostname(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Hostname(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestUUID(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid UUID lowercase", "550e8400-e29b-41d4-a716-446655440000", false},
		{"valid UUID uppercase", "550E8400-E29B-41D4-A716-446655440000", false},
		{"valid UUID mixed case", "550e8400-E29b-41d4-A716-446655440000", false},
		{"empty UUID", "", true},
		{"missing hyphens", "550e8400e29b41d4a716446655440000", true},
		{"wrong format", "550e8400-e29b-41d4-a716", true},
		{"invalid characters", "550e8400-e29b-41d4-a716-44665544000g", true},
		{"too short", "550e8400-e29b-41d4-a716-44665544000", true},
		{"too long", "550e8400-e29b-41d4-a716-4466554400000", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := UUID(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("UUID(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestNotEmpty(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"non-empty", "value", false},
		{"empty", "", true},
		{"whitespace only", "   ", true},
		{"tab only", "\t", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NotEmpty(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("NotEmpty(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestMinLength(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		min     int
		wantErr bool
	}{
		{"meets minimum", "abc", 3, false},
		{"exceeds minimum", "abcd", 3, false},
		{"below minimum", "ab", 3, true},
		{"empty with min 0", "", 0, false},
		{"empty with min 1", "", 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := MinLength(tt.min)
			err := validator(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("MinLength(%d)(%q) error = %v, wantErr %v", tt.min, tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestMaxLength(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		max     int
		wantErr bool
	}{
		{"meets maximum", "abc", 3, false},
		{"below maximum", "ab", 3, false},
		{"exceeds maximum", "abcd", 3, true},
		{"empty", "", 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := MaxLength(tt.max)
			err := validator(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("MaxLength(%d)(%q) error = %v, wantErr %v", tt.max, tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestLength(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		exact   int
		wantErr bool
	}{
		{"exact length", "abc", 3, false},
		{"too short", "ab", 3, true},
		{"too long", "abcd", 3, true},
		{"empty with exact 0", "", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := Length(tt.exact)
			err := validator(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Length(%d)(%q) error = %v, wantErr %v", tt.exact, tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestRegex(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		pattern string
		wantErr bool
	}{
		{"matches pattern", "abc123", "^[a-z]+[0-9]+$", false},
		{"doesn't match", "123abc", "^[a-z]+[0-9]+$", true},
		{"empty matches empty pattern", "", "^$", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := Regex(tt.pattern)
			err := validator(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Regex(%q)(%q) error = %v, wantErr %v", tt.pattern, tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestAll(t *testing.T) {
	validator := All(
		NotEmpty,
		MinLength(3),
		MaxLength(10),
	)

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"passes all", "abc", false},
		{"fails NotEmpty", "", true},
		{"fails MinLength", "ab", true},
		{"fails MaxLength", "abcdefghijk", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("All(...)(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestAny(t *testing.T) {
	validator := Any(
		IPv4,
		IPv6,
	)

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"passes IPv4", "192.168.1.1", false},
		{"passes IPv6", "2001:db8::1", false},
		{"fails both", "not an ip", true},
		{"fails empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Any(...)(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestInteger(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid integer", "123", false},
		{"valid negative", "-456", false},
		{"valid zero", "0", false},
		{"valid large", "2147483647", false},
		{"empty", "", true},
		{"not a number", "abc", true},
		{"float", "123.45", true},
		{"with spaces", " 123 ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Integer(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Integer(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestInt64(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid int64", "123", false},
		{"valid negative", "-456", false},
		{"valid zero", "0", false},
		{"valid large", "9223372036854775807", false},
		{"valid negative large", "-9223372036854775808", false},
		{"empty", "", true},
		{"not a number", "abc", true},
		{"float", "123.45", true},
		{"too large", "9223372036854775808", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Int64(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Int64(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestFloat64(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid float", "123.45", false},
		{"valid negative", "-456.78", false},
		{"valid zero", "0", false},
		{"valid integer", "123", false},
		{"valid scientific", "1.23e10", false},
		{"valid negative scientific", "-1.23e-10", false},
		{"empty", "", true},
		{"not a number", "abc", true},
		{"with spaces", " 123.45 ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Float64(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Float64(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestIntRange(t *testing.T) {
	validator := IntRange(1, 100)

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"within range", "50", false},
		{"at minimum", "1", false},
		{"at maximum", "100", false},
		{"below minimum", "0", true},
		{"above maximum", "101", true},
		{"not a number", "abc", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("IntRange(1, 100)(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestFloatRange(t *testing.T) {
	validator := FloatRange(0.0, 1.0)

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"within range", "0.5", false},
		{"at minimum", "0.0", false},
		{"at maximum", "1.0", false},
		{"below minimum", "-0.1", true},
		{"above maximum", "1.1", true},
		{"not a number", "abc", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("FloatRange(0.0, 1.0)(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}
