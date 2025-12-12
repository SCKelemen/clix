package clix

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigManager loads and stores configuration from YAML files and environment variables.
// Configuration values are automatically loaded when App.Run is called
// and are accessible via Context getters with precedence: command flags > app flags > env > config > defaults.
// The config file uses YAML format. Nested structures are flattened using dot notation
// (e.g., "project.name" for nested "project: name: value").
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
	// ConfigInt stores 32-bit integers.
	ConfigInt
	// ConfigInt64 stores 64-bit integers.
	ConfigInt64
	// ConfigFloat64 stores floating-point numbers.
	ConfigFloat64
)

// ConfigSchemaOption configures a config schema using the functional options pattern.
// Options can be used to build schemas:
//
//	// Using functional options
//	app.Config.RegisterSchema(
//		WithConfigKey("project.retries"),
//		WithConfigType(clix.ConfigInt),
//		WithConfigValidate(validation.IntRange(1, 10)),
//	)
//
//	// Using struct (primary API)
//	app.Config.RegisterSchema(clix.ConfigSchema{
//		Key:  "project.retries",
//		Type: clix.ConfigInt,
//		Validate: validation.IntRange(1, 10),
//	})
type ConfigSchemaOption interface {
	// ApplyConfigSchema configures a config schema struct.
	// Exported so extension packages can implement ConfigSchemaOption.
	ApplyConfigSchema(*ConfigSchema)
}

// ConfigSchema describes an expected type (and optional validator) for a config key.
// This struct implements ConfigSchemaOption, so it can be used alongside functional options.
// This is optional— schemas only apply when registered via RegisterSchema.
//
// Example:
//
//	// Struct-based (primary API)
//	app.Config.RegisterSchema(clix.ConfigSchema{
//		Key:  "project.retries",
//		Type: clix.ConfigInt,
//		Validate: validation.IntRange(1, 10),
//	})
//
//	// Functional options
//	app.Config.RegisterSchema(
//		WithConfigKey("project.retries"),
//		WithConfigType(clix.ConfigInt),
//		WithConfigValidate(validation.IntRange(1, 10)),
//	)
type ConfigSchema struct {
	Key      string
	Type     ConfigType
	Validate func(string) error
}

// ApplyConfigSchema implements ConfigSchemaOption so ConfigSchema can be used directly.
func (s ConfigSchema) ApplyConfigSchema(schema *ConfigSchema) {
	if s.Key != "" {
		schema.Key = s.Key
	}
	if s.Type != ConfigString {
		schema.Type = s.Type
	}
	if s.Validate != nil {
		schema.Validate = s.Validate
	}
}

// NewConfigManager constructs a manager for the given application name.
func NewConfigManager(name string) *ConfigManager {
	return &ConfigManager{
		values:  make(map[string]string),
		schemas: make(map[string]ConfigSchema),
	}
}

// Load reads configuration from the provided path. Missing files are ignored.
// The file format is YAML. Nested structures are flattened using dot notation.
func (m *ConfigManager) Load(path string) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	var data map[string]interface{}
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return fmt.Errorf("failed to parse config file as YAML: %w", err)
	}

	if m.values == nil {
		m.values = make(map[string]string)
	}
	flattenYAML("", data, m.values)
	return nil
}

// flattenYAML recursively flattens a nested YAML structure into dot-notation keys.
func flattenYAML(prefix string, data map[string]interface{}, result map[string]string) {
	for key, value := range data {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch v := value.(type) {
		case map[string]interface{}:
			// Recurse into nested maps
			flattenYAML(fullKey, v, result)
		case []interface{}:
			// For arrays, convert to comma-separated string
			parts := make([]string, len(v))
			for i, item := range v {
				parts[i] = fmt.Sprintf("%v", item)
			}
			result[fullKey] = strings.Join(parts, ",")
		default:
			// Convert value to string
			result[fullKey] = fmt.Sprintf("%v", v)
		}
	}
}

// Save writes the configuration to the provided path in YAML format.
// Dot-notation keys (e.g., "project.name") are saved as flat key:value pairs.
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

	// Convert flat map[string]string to map[string]interface{} for YAML encoding
	data := make(map[string]interface{})
	for k, v := range m.values {
		data[k] = v
	}

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)
	if err := encoder.Encode(data); err != nil {
		file.Close()
		return err
	}

	if err := file.Close(); err != nil {
		return err
	}

	return os.Rename(tmp, path)
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
// Accepts either ConfigSchema structs (primary API) or functional options (convenience layer).
//
// Example - two API styles:
//
//	// 1. Struct-based (primary API)
//	app.Config.RegisterSchema(clix.ConfigSchema{
//		Key:  "project.retries",
//		Type: clix.ConfigInt,
//		Validate: validation.IntRange(1, 10),
//	})
//
//	// 2. Functional options
//	app.Config.RegisterSchema(
//		clix.WithConfigKey("project.retries"),
//		clix.WithConfigType(clix.ConfigInt),
//		clix.WithConfigValidate(validation.IntRange(1, 10)),
//	)
//
//	// 3. Mixed (struct + functional options)
//	app.Config.RegisterSchema(
//		clix.ConfigSchema{Key: "project.retries"},
//		clix.WithConfigType(clix.ConfigInt),
//	)
func (m *ConfigManager) RegisterSchema(entries ...ConfigSchemaOption) {
	if m.schemas == nil {
		m.schemas = make(map[string]ConfigSchema)
	}
	for _, entry := range entries {
		var schema ConfigSchema
		switch v := entry.(type) {
		case ConfigSchema:
			schema = v
		default:
			entry.ApplyConfigSchema(&schema)
		}
		if schema.Key == "" {
			continue
		}
		m.schemas[schema.Key] = schema
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
	case ConfigInt:
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

// Int retrieves an int value from persisted config.
func (m *ConfigManager) Int(key string) (int, bool) {
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

// Functional option helpers for config schemas

// WithConfigKey sets the config schema key.
func WithConfigKey(key string) ConfigSchemaOption {
	return configKeyOption(key)
}

// WithConfigType sets the config schema type.
func WithConfigType(typ ConfigType) ConfigSchemaOption {
	return configTypeOption{typ: typ}
}

// WithConfigValidate sets the config schema validation function.
func WithConfigValidate(validate func(string) error) ConfigSchemaOption {
	return configValidateOption{validate: validate}
}

// Internal option types

type configKeyOption string

func (o configKeyOption) ApplyConfigSchema(schema *ConfigSchema) {
	schema.Key = string(o)
}

type configTypeOption struct {
	typ ConfigType
}

func (o configTypeOption) ApplyConfigSchema(schema *ConfigSchema) {
	schema.Type = o.typ
}

type configValidateOption struct {
	validate func(string) error
}

func (o configValidateOption) ApplyConfigSchema(schema *ConfigSchema) {
	schema.Validate = o.validate
}
