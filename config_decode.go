package clix

import (
	"fmt"
	"maps"
	"slices"
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

// subtree uses the same YAML representation for strict decoding and saving.
func (m *ConfigManager) subtree(prefix string, c *yamlConverter) (*jsonNode, error) {
	var keys []string
	for k := range m.nodes {
		if prefix == "" || k == prefix || strings.HasPrefix(k, prefix+".") {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return nil, nil
	}
	n, err := m.buildYAML(prefix, keys)
	if err != nil {
		return nil, err
	}
	return c.convert(n, "", nil)
}

func (m *ConfigManager) buildYAML(prefix string, keys []string) (*yaml.Node, error) {
	slices.Sort(keys)
	root := &configTree{}
	for _, k := range keys {
		rel := k
		if prefix != "" {
			rel = strings.TrimPrefix(strings.TrimPrefix(k, prefix), ".")
		}
		t := root
		if rel != "" {
			depth := 0
			for seg := range strings.SplitSeq(rel, ".") {
				depth++
				if depth > maxYAMLDepth {
					return nil, fmt.Errorf("%s: config key exceeds %d levels", m.source(), maxYAMLDepth)
				}
				if t.leaf != nil && t.leaf.Kind != yaml.MappingNode {
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
		t.leaf = m.nodes[k]
	}
	return m.treeYAML(root, prefix), nil
}

// mixedKeyError reports a key that holds a value and also keys under it,
// which only Set or loading several files can cause.
func (m *ConfigManager) mixedKeyError(key string) error {
	return fmt.Errorf("%s: key %q holds a value and also keys under it", m.source(), key)
}

func (m *ConfigManager) treeYAML(t *configTree, full string) *yaml.Node {
	if len(t.children) == 0 {
		return t.leaf
	}
	names := slices.Sorted(maps.Keys(t.children))
	node := &yaml.Node{Kind: yaml.MappingNode}
	if m.seqs[full] && isIndexList(names) {
		node.Kind = yaml.SequenceNode
		for i := range names {
			names[i] = strconv.Itoa(i)
		}
	}
	for _, name := range names {
		if node.Kind == yaml.MappingNode {
			node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name})
		}
		path := name
		if full != "" {
			path = full + "." + name
		}
		node.Content = append(node.Content, m.treeYAML(t.children[name], path))
	}
	return node
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
