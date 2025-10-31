package validation

import (
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
