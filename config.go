package clix

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
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
	numbers map[string]string // decimal form of YAML numbers written as 0x1F, 0755, 1e3, ...
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

// Load reads configuration from the provided path. Missing and empty files
// are ignored. The file format is YAML. Nested structures are flattened
// using dot notation, and null values (for example, "project:" with nothing
// after it) are skipped, so they never count as configured.
//
// Scalars keep the text written in the file: "tag: 1.10" reads "1.10",
// "mode: 0755" reads "0755" and "date: 2024-01-02" reads "2024-01-02".
// Only int and float flags (and the Int, Int64 and Float64 getters) see
// YAML's numeric reading, so "mode: 0755" sets an int flag to 493 and
// "mask: 0x1F" sets it to 31. Map keys are used as written, so "80: http"
// under "ports" is "ports.80". A list of scalars is joined with commas; a
// list that holds maps or lists is flattened with indexes ("hosts.0.name").
//
// A key that appears twice after flattening, such as "project.name" next
// to a nested "project: {name: ...}", is an error, because the winner would
// otherwise be arbitrary. Keys pulled in with a YAML merge key ("<<") may
// be overridden. Errors name the file.
func (m *ConfigManager) Load(path string) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	var doc yaml.Node
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			// An empty file (or one holding only comments) has no documents.
			return nil
		}
		return fmt.Errorf("failed to parse config file %s as YAML: %w", path, err)
	}

	f := flattener{
		values:  make(map[string]string),
		numbers: make(map[string]string),
		weak:    make(map[string]bool),
	}
	if err := f.document(&doc); err != nil {
		return fmt.Errorf("config file %s: %w", path, err)
	}

	if m.values == nil {
		m.values = make(map[string]string)
	}
	if m.numbers == nil {
		m.numbers = make(map[string]string)
	}
	for k, v := range f.values {
		m.values[k] = v
		if n, ok := f.numbers[k]; ok {
			m.numbers[k] = n
		} else {
			delete(m.numbers, k)
		}
	}
	return nil
}

// flattener turns a YAML document into dot-notation keys, keeping each
// scalar's text as written.
type flattener struct {
	values  map[string]string
	numbers map[string]string // decimal form of numeric scalars
	weak    map[string]bool   // keys set through a merge key, which later keys may override
}

// document flattens the top-level mapping of doc.
func (f *flattener) document(doc *yaml.Node) error {
	n := doc
	if n.Kind == yaml.DocumentNode {
		if len(n.Content) == 0 {
			return nil
		}
		n = n.Content[0]
	}
	n = resolveAlias(n)
	if n.Kind == yaml.ScalarNode && n.ShortTag() == "!!null" {
		return nil
	}
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: the top level must be a mapping of keys to values", n.Line)
	}
	return f.mapping("", n, false)
}

// mapping flattens the pairs of n under prefix. Merge keys ("<<") are
// applied first, so the mapping's own keys override what they bring in.
func (f *flattener) mapping(prefix string, n *yaml.Node, weak bool) error {
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, value := n.Content[i], resolveAlias(n.Content[i+1])
		if key.ShortTag() != "!!merge" {
			continue
		}
		sources := []*yaml.Node{value}
		if value.Kind == yaml.SequenceNode {
			sources = value.Content
		}
		for _, src := range sources {
			src = resolveAlias(src)
			if src.Kind != yaml.MappingNode {
				return fmt.Errorf("line %d: a merge key (<<) needs a mapping", src.Line)
			}
			if err := f.mapping(prefix, src, true); err != nil {
				return err
			}
		}
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, value := resolveAlias(n.Content[i]), n.Content[i+1]
		if key.ShortTag() == "!!merge" {
			continue
		}
		if key.Kind != yaml.ScalarNode {
			return fmt.Errorf("line %d: keys must be scalars", key.Line)
		}
		full := key.Value
		if prefix != "" {
			full = prefix + "." + key.Value
		}
		if err := f.value(full, value, weak); err != nil {
			return err
		}
	}
	return nil
}

// value flattens one value under key.
func (f *flattener) value(key string, n *yaml.Node, weak bool) error {
	n = resolveAlias(n)
	switch n.Kind {
	case yaml.ScalarNode:
		if n.ShortTag() == "!!null" {
			// "key:" with no value is YAML null; treat it as unset.
			return nil
		}
		return f.set(key, n.Value, yamlNumber(n), weak, n.Line)
	case yaml.MappingNode:
		return f.mapping(key, n, weak)
	case yaml.SequenceNode:
		scalars := true
		for _, item := range n.Content {
			if resolveAlias(item).Kind != yaml.ScalarNode {
				scalars = false
				break
			}
		}
		if scalars {
			// A list of scalars is joined with commas.
			parts := make([]string, len(n.Content))
			for i, item := range n.Content {
				if item = resolveAlias(item); item.ShortTag() != "!!null" {
					parts[i] = item.Value
				}
			}
			return f.set(key, strings.Join(parts, ","), "", weak, n.Line)
		}
		for i, item := range n.Content {
			if err := f.value(key+"."+strconv.Itoa(i), item, weak); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("line %d: unsupported value for %q", n.Line, key)
	}
}

// set records a flattened value, rejecting a key that is already set
// unless one of the two came from a merge key.
func (f *flattener) set(key, value, number string, weak bool, line int) error {
	if _, exists := f.values[key]; exists {
		switch {
		case weak:
			// An explicit key, or an earlier merge source, wins.
			return nil
		case !f.weak[key]:
			return fmt.Errorf("line %d: key %q is defined more than once", line, key)
		}
	}
	f.values[key] = value
	f.weak[key] = weak
	if number != "" {
		f.numbers[key] = number
	} else {
		delete(f.numbers, key)
	}
	return nil
}

// resolveAlias follows YAML aliases (*name) to the node they refer to.
func resolveAlias(n *yaml.Node) *yaml.Node {
	for i := 0; n.Kind == yaml.AliasNode && n.Alias != nil && i < 100; i++ {
		n = n.Alias
	}
	return n
}

// yamlNumber returns the decimal form of a YAML int or float scalar, or ""
// for any other scalar.
func yamlNumber(n *yaml.Node) string {
	tag := n.ShortTag()
	if tag != "!!int" && tag != "!!float" {
		return ""
	}
	var v interface{}
	if err := n.Decode(&v); err != nil {
		return ""
	}
	switch x := v.(type) {
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	case float64:
		if x == math.Trunc(x) && math.Abs(x) < 1e21 {
			return strconv.FormatFloat(x, 'f', -1, 64)
		}
		return strconv.FormatFloat(x, 'g', -1, 64)
	}
	return ""
}

// number returns the decimal form of key's value when the config file
// wrote it as a YAML number in another notation.
func (m *ConfigManager) number(key string) (string, bool) {
	v, ok := m.numbers[key]
	return v, ok
}

// numeric returns the text to parse as a number for key: YAML's numeric
// reading when the file held a number, otherwise the stored value.
func (m *ConfigManager) numeric(key string) (string, bool) {
	if v, ok := m.numbers[key]; ok {
		return v, true
	}
	v, ok := m.values[key]
	return v, ok
}

// Save writes the configuration to the provided path in YAML format.
// Dot-notation keys (e.g., "project.name") are saved as flat key:value pairs.
//
// Config files often hold credentials or project identifiers, so a missing
// directory is created with mode 0700 and the file is written with mode
// 0600. The file is replaced atomically: the YAML is written to a
// temporary file in the same directory, synced, and renamed over the old
// file. When path is a symbolic link (for example, into a dotfiles
// repository), the file it points to is replaced and the link is kept.
func (m *ConfigManager) Save(path string) error {
	if m.values == nil {
		return nil
	}

	target, err := resolveSymlinks(path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	// os.CreateTemp creates the file with mode 0600.
	file, err := os.CreateTemp(dir, filepath.Base(target)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := file.Name()
	if err := m.writeYAML(file); err != nil {
		file.Close()
		os.Remove(tmp)
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(tmp)
		return err
	}
	if err := file.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// resolveSymlinks follows path while it is a symbolic link and returns the
// file it finally names, which need not exist yet.
func resolveSymlinks(path string) (string, error) {
	current := path
	for range 255 {
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return current, nil
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return current, nil
		}
		link, err := os.Readlink(current)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(link) {
			link = filepath.Join(filepath.Dir(current), link)
		}
		current = link
	}
	return "", fmt.Errorf("config file %s: too many levels of symbolic links", path)
}

// writeYAML encodes the values as a flat YAML mapping.
func (m *ConfigManager) writeYAML(w io.Writer) error {
	// Convert flat map[string]string to map[string]interface{} for YAML encoding
	data := make(map[string]interface{}, len(m.values))
	for k, v := range m.values {
		data[k] = v
	}

	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	if err := encoder.Encode(data); err != nil {
		_ = encoder.Close()
		return err
	}
	// Close flushes any output the encoder is still buffering.
	return encoder.Close()
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
	delete(m.numbers, key)
}

// Delete removes a key from the configuration. It returns true if the key existed.
// Keys are stored using dot-separated paths (e.g. "project.default").
func (m *ConfigManager) Delete(key string) bool {
	if m.values == nil {
		return false
	}
	if _, ok := m.values[key]; ok {
		delete(m.values, key)
		delete(m.numbers, key)
		return true
	}
	return false
}

// Reset removes all values.
func (m *ConfigManager) Reset() {
	m.values = make(map[string]string)
	m.numbers = nil
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
//
// Functional options and struct fields that follow a key apply to that key,
// so WithConfigKey("a"), WithConfigType(ConfigInt) registers one schema. A
// new key (from WithConfigKey or a ConfigSchema with a different Key)
// starts the next schema, which is how several schemas are registered in
// one call.
func (m *ConfigManager) RegisterSchema(entries ...ConfigSchemaOption) {
	if m.schemas == nil {
		m.schemas = make(map[string]ConfigSchema)
	}

	var current ConfigSchema
	flush := func() {
		if current.Key != "" {
			m.schemas[current.Key] = current
		}
		current = ConfigSchema{}
	}

	for _, entry := range entries {
		if entry == nil {
			continue
		}
		if p, ok := entry.(*ConfigSchema); ok {
			if p == nil {
				continue
			}
			entry = *p
		}

		if v, ok := entry.(ConfigSchema); ok && v.Key != "" {
			// A complete struct describes its own key. Options given before
			// any key carry over to it unless the struct sets them itself.
			pending := current
			if pending.Key != "" {
				flush()
				pending = ConfigSchema{}
			}
			current = v
			if current.Type == ConfigString {
				current.Type = pending.Type
			}
			if current.Validate == nil {
				current.Validate = pending.Validate
			}
			continue
		}

		var probe ConfigSchema
		entry.ApplyConfigSchema(&probe)
		if probe.Key != "" && current.Key != "" && probe.Key != current.Key {
			flush()
		}
		entry.ApplyConfigSchema(&current)
	}
	flush()
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

// Int retrieves an int value from persisted config. A YAML number in
// another notation, such as 0x1F, reads as its value (31).
func (m *ConfigManager) Int(key string) (int, bool) {
	value, ok := m.numeric(key)
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
	value, ok := m.numeric(key)
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
	value, ok := m.numeric(key)
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
