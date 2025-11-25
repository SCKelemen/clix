package clix

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ConfigManager loads and stores configuration from YAML files and environment
// variables. Configuration values are automatically loaded when App.Run is called
// and are accessible via Context getters with precedence: command flags > app flags > env > config > defaults.
//
// Example:
//
//	app := clix.NewApp("myapp")
//	// Config is automatically created and loaded
//	// Access values via context:
//	cmd.Run = func(ctx *clix.Context) error {
//		if value, ok := ctx.String("key"); ok {
//			// Value from config file, env var, or default
//		}
//		return nil
//	}
type ConfigManager struct {
	values  map[string]string
	schemas map[string]ConfigSchema
}

// ConfigType represents the desired type for a configuration value.
type ConfigType int

const (
	// ConfigString stores raw string values (default behaviour).
	ConfigString ConfigType = iota
	// ConfigBool stores canonical boolean values ("true"/"false").
	ConfigBool
	// ConfigInteger stores 32-bit integers.
	ConfigInteger
	// ConfigInt64 stores 64-bit integers.
	ConfigInt64
	// ConfigFloat64 stores floating-point numbers.
	ConfigFloat64
)

// ConfigSchema describes an expected type (and optional validator) for a config key.
// This is optional— schemas only apply when registered via RegisterSchema.
type ConfigSchema struct {
	Key      string
	Type     ConfigType
	Validate func(string) error
}

// NewConfigManager constructs a manager for the given application name.
func NewConfigManager(name string) *ConfigManager {
	return &ConfigManager{
		values:  make(map[string]string),
		schemas: make(map[string]ConfigSchema),
	}
}

// Load reads configuration from the provided path. Missing files are ignored.
func (m *ConfigManager) Load(path string) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, "\"'")
		m.values[key] = value
	}

	return scanner.Err()
}

// Save writes the configuration to the provided path in YAML format.
func (m *ConfigManager) Save(path string) error {
	if m.values == nil {
		return nil
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp := path + ".tmp"
	file, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer file.Close()

	keys := make([]string, 0, len(m.values))
	for k := range m.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if _, err := fmt.Fprintf(file, "%s: %s\n", key, quoteIfNeeded(m.values[key])); err != nil {
			return err
		}
	}

	if err := file.Close(); err != nil {
		return err
	}

	return os.Rename(tmp, path)
}

func quoteIfNeeded(value string) string {
	if strings.ContainsAny(value, ":#") || strings.HasPrefix(value, " ") || strings.HasSuffix(value, " ") {
		return fmt.Sprintf("%q", value)
	}
	return value
}

// Get retrieves a value.
func (m *ConfigManager) Get(key string) (string, bool) {
	value, ok := m.values[key]
	return value, ok
}

// Set stores a value.
func (m *ConfigManager) Set(key, value string) {
	if m.values == nil {
		m.values = make(map[string]string)
	}
	m.values[key] = value
}

// Delete removes a key from the configuration. It returns true if the key existed.
// Keys are stored using dot-separated paths (e.g. "project.default").
func (m *ConfigManager) Delete(key string) bool {
	if m.values == nil {
		return false
	}
	if _, ok := m.values[key]; ok {
		delete(m.values, key)
		return true
	}
	return false
}

// Reset removes all values.
func (m *ConfigManager) Reset() {
	m.values = make(map[string]string)
}

// Values returns a copy of the stored values.
func (m *ConfigManager) Values() map[string]string {
	copy := make(map[string]string, len(m.values))
	for k, v := range m.values {
		copy[k] = v
	}
	return copy
}

// RegisterSchema registers one or more schema entries for configuration keys.
// Registration is optional; keys without schema entries behave like raw strings.
func (m *ConfigManager) RegisterSchema(entries ...ConfigSchema) {
	if m.schemas == nil {
		m.schemas = make(map[string]ConfigSchema)
	}
	for _, entry := range entries {
		if entry.Key == "" {
			continue
		}
		m.schemas[entry.Key] = entry
	}
}

// NormalizeValue validates and canonicalises a value according to the schema (if present).
// The returned string is safe to persist. When no schema exists, the original value is returned.
func (m *ConfigManager) NormalizeValue(key, value string) (string, error) {
	entry, ok := m.schemas[key]
	if !ok {
		// No schema registered; still run validator if present (unlikely) but keep as-is.
		if entry.Validate != nil {
			if err := entry.Validate(value); err != nil {
				return "", err
			}
		}
		return value, nil
	}

	switch entry.Type {
	case ConfigBool:
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return "", fmt.Errorf("expected boolean for %q: %w", key, err)
		}
		value = strconv.FormatBool(parsed)
	case ConfigInteger:
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return "", fmt.Errorf("expected integer for %q: %w", key, err)
		}
		value = strconv.Itoa(parsed)
	case ConfigInt64:
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil {
			return "", fmt.Errorf("expected int64 for %q: %w", key, err)
		}
		value = strconv.FormatInt(parsed, 10)
	case ConfigFloat64:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return "", fmt.Errorf("expected float64 for %q: %w", key, err)
		}
		value = strconv.FormatFloat(parsed, 'f', -1, 64)
	default: // ConfigString or unknown
		value = strings.TrimSuffix(value, "\n")
	}

	if entry.Validate != nil {
		if err := entry.Validate(value); err != nil {
			return "", err
		}
	}

	return value, nil
}

// String retrieves a raw string value directly from persisted config.
func (m *ConfigManager) String(key string) (string, bool) {
	value, ok := m.values[key]
	return value, ok
}

// Bool retrieves a boolean value from persisted config.
func (m *ConfigManager) Bool(key string) (bool, bool) {
	value, ok := m.values[key]
	if !ok {
		return false, false
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, false
	}
	return parsed, true
}

// Integer retrieves an int value from persisted config.
func (m *ConfigManager) Integer(key string) (int, bool) {
	value, ok := m.values[key]
	if !ok {
		return 0, false
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, false
	}
	return parsed, true
}

// Int64 retrieves an int64 value from persisted config.
func (m *ConfigManager) Int64(key string) (int64, bool) {
	value, ok := m.values[key]
	if !ok {
		return 0, false
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

// Float64 retrieves a float64 value from persisted config.
func (m *ConfigManager) Float64(key string) (float64, bool) {
	value, ok := m.values[key]
	if !ok {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}
