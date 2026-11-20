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
	numbers map[string]string     // decimal form of YAML numbers written as 0x1F, 0755, 1e3, ...
	lists   map[string][]string   // the items of YAML lists of scalars, for slice flags
	nodes   map[string]*yaml.Node // values for Save and Decode, including containers with no scalar leaves
	seqs    map[string]bool       // keys that held YAML lists of maps or lists, flattened with indexes (see Decode)
	schemas map[string]ConfigSchema
	path    string // the file last loaded, for messages
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
//
// Save writes a loaded key that was not changed since exactly as it was
// written (0x1F stays a hex number, "0755" a string, [a, "b,c"] a list of
// two items), so saving never changes what a key reads.
func (m *ConfigManager) Load(path string) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	// Opened only for reading, so a Close error loses nothing.
	defer func() { _ = file.Close() }()

	var doc yaml.Node
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			// An empty file (or one holding only comments) has no documents.
			return nil
		}
		return fmt.Errorf("failed to parse config file %s as YAML: %w", path, err)
	}

	if err := checkYAMLGraph(&doc); err != nil {
		return fmt.Errorf("config file %s: %w", path, err)
	}

	f := flattener{
		values:  make(map[string]string),
		numbers: make(map[string]string),
		lists:   make(map[string][]string),
		nodes:   make(map[string]*yaml.Node),
		weak:    make(map[string]bool),
		seqs:    make(map[string]bool),
	}
	if err := f.document(&doc); err != nil {
		return fmt.Errorf("config file %s: %w", path, err)
	}
	m.path = path
	if m.seqs == nil {
		m.seqs = make(map[string]bool)
	}
	for k := range f.seqs {
		m.seqs[k] = true
	}

	if m.values == nil {
		m.values = make(map[string]string)
	}
	if m.numbers == nil {
		m.numbers = make(map[string]string)
	}
	if m.lists == nil {
		m.lists = make(map[string][]string)
	}
	if m.nodes == nil {
		m.nodes = make(map[string]*yaml.Node)
	}
	for k, n := range f.nodes {
		// A structure-only node does not overwrite a scalar from an earlier
		// file. Decode reports a scalar with descendants as a mixed key.
		_, oldValue := m.values[k]
		if _, newValue := f.values[k]; newValue || !oldValue {
			m.nodes[k] = n
		}
	}
	for k, v := range f.values {
		m.values[k] = v
		if n, ok := f.numbers[k]; ok {
			m.numbers[k] = n
		} else {
			delete(m.numbers, k)
		}
		if items, ok := f.lists[k]; ok {
			m.lists[k] = items
		} else {
			delete(m.lists, k)
		}
	}
	return nil
}

// flattener turns a YAML document into dot-notation keys, keeping each
// scalar's text as written.
type flattener struct {
	values  map[string]string
	numbers map[string]string   // decimal form of numeric scalars
	lists   map[string][]string // items of lists of scalars
	nodes   map[string]*yaml.Node
	weak    map[string]bool // keys set through a merge key, which later keys may override
	seqs    map[string]bool // keys of lists flattened with indexes
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

// mapping flattens the pairs of n under prefix.
func (f *flattener) mapping(prefix string, n *yaml.Node, weak bool) error {
	pairs, err := mappingPairs(n, 0)
	if err != nil {
		return err
	}
	for _, p := range pairs {
		full := p.key
		if prefix != "" {
			full = prefix + "." + p.key
		}
		if err := f.value(full, p.value, weak || p.merged); err != nil {
			return err
		}
	}
	return nil
}

// yamlPair is one key of a mapping with its value. merged marks a pair
// that came from a merge key (<<).
type yamlPair struct {
	key    string
	value  *yaml.Node
	merged bool
}

// mappingPairs returns the pairs of mapping n with merge keys applied, as
// the YAML merge-key spec says (https://yaml.org/type/merge.html): a merged
// mapping only adds keys that n does not have itself, and with a list of
// mappings to merge, earlier ones win over later ones. The merge is
// shallow: a key of n, even one set to null, replaces the merged value as
// a whole, so "child: {<<: *base, y: {w: 3}}" does not keep base's y.z.
func mappingPairs(n *yaml.Node, depth int) ([]yamlPair, error) {
	if depth > 100 {
		return nil, fmt.Errorf("line %d: merge keys nested too deeply", n.Line)
	}
	var pairs []yamlPair
	var sources []*yaml.Node
	own := make(map[string]bool)
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, value := resolveAlias(n.Content[i]), n.Content[i+1]
		if key.ShortTag() == "!!merge" {
			v := resolveAlias(value)
			if v.Kind == yaml.SequenceNode {
				for _, item := range v.Content {
					sources = append(sources, resolveAlias(item))
				}
			} else {
				sources = append(sources, v)
			}
			continue
		}
		if key.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("line %d: keys must be scalars", key.Line)
		}
		pairs = append(pairs, yamlPair{key: key.Value, value: value})
		own[key.Value] = true
	}
	for _, src := range sources {
		if src.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("line %d: a merge key (<<) needs a mapping", src.Line)
		}
		merged, err := mappingPairs(src, depth+1)
		if err != nil {
			return nil, err
		}
		for _, p := range merged {
			if own[p.key] {
				continue
			}
			own[p.key] = true
			pairs = append(pairs, yamlPair{key: p.key, value: p.value, merged: true})
		}
	}
	return pairs, nil
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
		return f.set(key, n.Value, yamlNumber(n), nil, n, weak)
	case yaml.MappingNode:
		if len(n.Content) == 0 {
			f.recordShape(key, &yaml.Node{Kind: yaml.MappingNode})
		}
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
			// A list of scalars reads as its items joined with commas;
			// slice flags get the items themselves.
			parts := make([]string, len(n.Content))
			for i, item := range n.Content {
				if item = resolveAlias(item); item.ShortTag() != "!!null" {
					parts[i] = item.Value
				}
			}
			return f.set(key, strings.Join(parts, ","), "", parts, n, weak)
		}
		f.seqs[key] = true
		for i, item := range n.Content {
			itemKey := key + "." + strconv.Itoa(i)
			// Keep the position even when the element has no flattened
			// leaves (an empty map, a map of unset keys, or a null).
			item = resolveAlias(item)
			switch {
			case item.Kind == yaml.MappingNode:
				f.recordShape(itemKey, &yaml.Node{Kind: yaml.MappingNode})
			case item.ShortTag() == "!!null":
				f.recordShape(itemKey, savedNode(item))
			}
			if err := f.value(itemKey, item, weak); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("line %d: unsupported value for %q", n.Line, key)
	}
}

// recordShape keeps an empty container or a list position without making it
// a scalar value visible through Get.
func (f *flattener) recordShape(key string, n *yaml.Node) {
	if _, scalar := f.values[key]; !scalar {
		f.nodes[key] = n
	}
}

// set records a flattened value read from node n, rejecting a key that
// is already set unless one of the two came from a merge key. items is
// non-nil for a list of scalars.
func (f *flattener) set(key, value, number string, items []string, n *yaml.Node, weak bool) error {
	if _, exists := f.values[key]; exists {
		switch {
		case weak:
			// An explicit key, or an earlier merge source, wins.
			return nil
		case !f.weak[key]:
			return fmt.Errorf("line %d: key %q is defined more than once", n.Line, key)
		}
	}
	f.values[key] = value
	f.nodes[key] = savedNode(n)
	f.weak[key] = weak
	if number != "" {
		f.numbers[key] = number
	} else {
		delete(f.numbers, key)
	}
	if items != nil {
		f.lists[key] = items
	} else {
		delete(f.lists, key)
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

// list returns the items of key's value when the config file wrote it as
// a YAML list of scalars.
func (m *ConfigManager) list(key string) ([]string, bool) {
	items, ok := m.lists[key]
	return items, ok
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
// Saving does not change what a key reads. A key loaded from a file and
// not changed since (with Set, Delete or Reset) is written as the file had
// it, with its YAML type, tag and quoting: "mask: 0x1F" stays a hex number
// that an int flag reads as 31, and "tags: [a, \"b,c\"]" a list of two
// items. A value given to Set is written plain when YAML reads that back as
// the same text and number ("retries: 5", "debug: true"), and quoted
// otherwise ("mode: \"0755\"", "version: \"1.10\""), so an int flag
// reads it as it reads the same text on the command line.
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
		_ = file.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// resolveSymlinks follows path while it is a symbolic link and returns the
// file it finally names, which need not exist yet: the file the kernel
// opens for path, so Save writes the file Load reads. It gives up after
// 255 links, as a loop would never end.
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
			// The kernel resolves a relative target from the directory the
			// link is physically in, so ".." goes up from there, not from
			// the path that led to the link (which may itself go through
			// symbolic links, such as ~/.config into a dotfiles tree).
			dir, err := filepath.EvalSymlinks(filepath.Dir(current))
			if err != nil {
				return "", err
			}
			link = filepath.Join(dir, link)
		}
		current = link
	}
	return "", fmt.Errorf("config file %s: too many levels of symbolic links", path)
}

// writeYAML keeps the flat file format for ordinary keys. Descendants
// of a structured sequence must be written together so Load can tell a
// list from a mapping whose keys happen to be numbers.
func (m *ConfigManager) writeYAML(w io.Writer) error {
	data := make(map[string]*yaml.Node)
	sequences := make(map[string][]string)
	for key, node := range m.nodes {
		if root, ok := m.sequenceAncestor(key); ok {
			sequences[root] = append(sequences[root], key)
		} else {
			data[key] = node
		}
	}
	for root, keys := range sequences {
		node, err := m.buildYAML(root, keys)
		if err != nil {
			return err
		}
		data[root] = node
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

// sequenceAncestor finds the outermost structured sequence at or above key.
func (m *ConfigManager) sequenceAncestor(key string) (string, bool) {
	for i := range len(key) {
		if key[i] == '.' && m.seqs[key[:i]] {
			return key[:i], true
		}
	}
	return key, m.seqs[key]
}

// savedNode returns a copy of the scalar or list of scalars n, with its
// kind, tag, style and text, for Save to write back. Aliases are resolved,
// and anchors and comments are left out, since keys are saved flat.
func savedNode(n *yaml.Node) *yaml.Node {
	n = resolveAlias(n)
	c := &yaml.Node{Kind: n.Kind, Tag: n.Tag, Style: n.Style, Value: n.Value}
	for _, item := range n.Content {
		c.Content = append(c.Content, savedNode(item))
	}
	return c
}

// newScalar returns the YAML scalar Save writes for value, a value given
// to Set: plain when YAML reads it back as the same text and, for a
// number, the same number (5, 1.5, true, hello), and otherwise a quoted
// string ("0755", "0x1F", "1.10", "", "null"), so the saved file reads
// exactly as the value did before Save.
func newScalar(value string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.ScalarNode, Value: value}
	switch n.ShortTag() {
	case "!!str", "!!bool", "!!timestamp":
	case "!!int", "!!float":
		if yamlNumber(n) != value {
			n.Tag = "!!str"
		}
	default:
		n.Tag = "!!str"
	}
	return n
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
	if m.nodes == nil {
		m.nodes = make(map[string]*yaml.Node)
	}
	m.values[key] = value
	m.nodes[key] = newScalar(value)
	delete(m.numbers, key)
	delete(m.lists, key)
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
		delete(m.lists, key)
		delete(m.nodes, key)
		return true
	}
	return false
}

// Reset removes all values.
func (m *ConfigManager) Reset() {
	m.values = make(map[string]string)
	m.numbers = nil
	m.lists = nil
	m.nodes = nil
	m.seqs = nil
	m.path = ""
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
