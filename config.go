package clix

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ConfigManager loads and stores configuration from YAML files and environment
// variables.
type ConfigManager struct {
	values map[string]string
}

// NewConfigManager constructs a manager for the given application name.
func NewConfigManager(name string) *ConfigManager {
	return &ConfigManager{values: make(map[string]string)}
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
