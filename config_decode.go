package clix

import (
	"encoding/json/jsontext"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Decode decodes the configuration under the dot-notation key prefix
// into v, a non-nil pointer, strictly, as UnmarshalStrict does: a key
// the target type has no field for is an error, names match exactly,
// and a value of the wrong type is an error. Every problem is reported in
// a *StrictError whose paths are config keys ("server.tls.cert_file",
// "hosts[1].port"), and when it is returned v is unchanged. Otherwise
// fields the configuration leaves out keep their values, so defaults set
// in v before the call survive.
//
//	type Server struct {
//		Host string        `json:"host"`
//		Port int           `json:"port"`
//		TLS  struct {
//			CertFile string `json:"cert_file"`
//		} `json:"tls"`
//		Timeout time.Duration `json:"timeout"`
//	}
//	server := Server{Port: 8080}
//	if err := app.Config.Decode("server", &server); err != nil {
//		return err
//	}
//
// An empty prefix decodes the whole configuration. A prefix that names a
// value rather than a section decodes that value. When nothing is under
// prefix, Decode returns nil and leaves v alone.
//
// Values keep the text written in the file, as Get returns it: "tag:
// 1.10" decodes into a string field as "1.10" and "mode: 0755" as "0755",
// while an int field reads YAML's number (493), as int flags do. A
// quoted string is never a number. Lists of maps decode into slices. Keys
// are read as Load flattened them, so "a.b: 1" and "a: {b: 1}" are the
// same. Values given to Set are read as Save would write them, so
// Set("port", "8080") decodes into an int field. Decode reads only the
// config file's values, not environment variables or flags.
func (m *ConfigManager) Decode(prefix string, v any) error {
	if err := checkTarget(v); err != nil {
		return err
	}
	c := newYAMLConverter(prefix)
	root, err := m.subtree(prefix, c)
	if err != nil {
		return err
	}
	if root == nil {
		return nil
	}
	return decodeTree(root, c, v, decodeOptions{source: m.source(), base: prefix})
}

// source names the configuration in messages.
func (m *ConfigManager) source() string {
	if m.path != "" {
		return "config file " + m.path
	}
	return "config"
}

// configTree is the configuration under a key, rebuilt from the
// flattened keys: a value, or keys under it.
type configTree struct {
	leaf     *yaml.Node
	children map[string]*configTree
}

// subtree rebuilds the configuration under prefix as JSON, or returns nil
// when there is none.
func (m *ConfigManager) subtree(prefix string, c *yamlConverter) (*jsonNode, error) {
	var keys []string
	for k := range m.values {
		if prefix == "" || k == prefix || strings.HasPrefix(k, prefix+".") {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return nil, nil
	}
	sort.Strings(keys)
	root := &configTree{}
	for _, k := range keys {
		rel := k
		if prefix != "" {
			rel = strings.TrimPrefix(strings.TrimPrefix(k, prefix), ".")
		}
		t := root
		if rel != "" {
			for _, seg := range strings.Split(rel, ".") {
				if t.leaf != nil {
					return nil, m.mixedKeyError(k)
				}
				if t.children == nil {
					t.children = make(map[string]*configTree)
				}
				child := t.children[seg]
				if child == nil {
					child = &configTree{}
					t.children[seg] = child
				}
				t = child
			}
		}
		if t.children != nil {
			return nil, m.mixedKeyError(k)
		}
		t.leaf = m.leafNode(k)
	}
	return m.treeJSON(root, prefix, c, "", nil)
}

// mixedKeyError reports a key that holds a value and also keys under it,
// which only Set or loading several files can cause.
func (m *ConfigManager) mixedKeyError(key string) error {
	return fmt.Errorf("%s: key %q holds a value and also keys under it", m.source(), key)
}

// leafNode returns the YAML node of key: the one it was loaded from, or,
// for a value given to Set, the scalar Save would write.
func (m *ConfigManager) leafNode(key string) *yaml.Node {
	if n, ok := m.nodes[key]; ok {
		return n
	}
	return newScalar(m.values[key])
}

// treeJSON converts t, the configuration under the key full, into JSON.
// Keys under a key that held a YAML list of maps or lists, named 0 to
// n-1, become a list again.
func (m *ConfigManager) treeJSON(t *configTree, full string, c *yamlConverter, ptr jsontext.Pointer, path []pathPart) (*jsonNode, error) {
	if t.leaf != nil {
		return c.convert(t.leaf, ptr, path)
	}
	names := make([]string, 0, len(t.children))
	for name := range t.children {
		names = append(names, name)
	}
	sort.Strings(names)
	join := func(name string) string {
		if full == "" {
			return name
		}
		return full + "." + name
	}
	if m.seqs[full] && isIndexList(names) {
		node := &jsonNode{kind: '['}
		for i := range names {
			name := strconv.Itoa(i)
			child, err := m.treeJSON(t.children[name], join(name), c, ptr.AppendToken(name), append(path[:len(path):len(path)], pathPart{index: i, isIndex: true}))
			if err != nil {
				return nil, err
			}
			node.items = append(node.items, child)
		}
		return node, nil
	}
	node := &jsonNode{kind: '{'}
	for _, name := range names {
		child, err := m.treeJSON(t.children[name], join(name), c, ptr.AppendToken(name), append(path[:len(path):len(path)], pathPart{name: name}))
		if err != nil {
			return nil, err
		}
		node.keys = append(node.keys, name)
		node.vals = append(node.vals, child)
	}
	return node, nil
}

// isIndexList reports whether names are exactly "0" to "n-1".
func isIndexList(names []string) bool {
	seen := make([]bool, len(names))
	for _, name := range names {
		i, err := strconv.Atoi(name)
		if err != nil || i < 0 || i >= len(names) || strconv.Itoa(i) != name || seen[i] {
			return false
		}
		seen[i] = true
	}
	return true
}
