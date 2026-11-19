package validation

import (
	"strings"
	"testing"
)

// NaN compares false against both bounds, so it passed every FloatRange.
func TestFloatRangeRejectsNaN(t *testing.T) {
	for _, v := range []string{"NaN", "nan", "-NaN"} {
		if err := FloatRange(0, 1)(v); err == nil {
			t.Errorf("FloatRange(0, 1)(%q) = nil", v)
		}
	}
	for v, ok := range map[string]bool{"0": true, "1": true, "0.5": true, "1.5": false, "Inf": false, "-Inf": false} {
		if err := FloatRange(0, 1)(v); (err == nil) != ok {
			t.Errorf("FloatRange(0, 1)(%q) = %v", v, err)
		}
	}
}

// The length validators counted bytes, so "héllo" (5 characters, 6 bytes)
// failed MaxLength(5) and "日本" (2 characters, 6 bytes) passed MinLength(3).
func TestLengthValidatorsCountCharacters(t *testing.T) {
	for _, tc := range []struct {
		name  string
		valid Validator
		value string
		ok    bool
	}{
		{"MaxLength(5) héllo", MaxLength(5), "héllo", true},
		{"MaxLength(4) héllo", MaxLength(4), "héllo", false},
		{"MinLength(3) 日本", MinLength(3), "日本", false},
		{"MinLength(2) 日本", MinLength(2), "日本", true},
		{"Length(2) 日本", Length(2), "日本", true},
		{"Length(6) 日本", Length(6), "日本", false},
		{"Length(3) abc", Length(3), "abc", true},
	} {
		if err := tc.valid(tc.value); (err == nil) != tc.ok {
			t.Errorf("%s = %v, want ok = %v", tc.name, err, tc.ok)
		}
	}
}

// A fully qualified name with the root's trailing dot is a valid hostname
// (RFC 1034 section 3.1).
func TestHostnameTrailingDot(t *testing.T) {
	label63 := strings.Repeat("a", 63)
	name253 := strings.Join([]string{label63, label63, label63, strings.Repeat("b", 61)}, ".")
	for v, ok := range map[string]bool{
		"example.com.":  true,
		"localhost.":    true,
		"example.com":   true,
		"example.com..": false,
		".":             false,
		"..":            false,
		".example.com":  false,
		name253:         true,
		name253 + ".":   true,
		name253 + "b":   false,
		name253 + "b.":  false,
	} {
		if err := Hostname(v); (err == nil) != ok {
			t.Errorf("Hostname(%q) = %v, want ok = %v", v, err, ok)
		}
	}
}
