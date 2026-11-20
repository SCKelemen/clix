package clix

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestCheckYAMLGraph(t *testing.T) {
	shared := &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{{Kind: yaml.ScalarNode, Value: "ok"}}}
	alias := &yaml.Node{Kind: yaml.AliasNode, Alias: shared}
	root := &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{shared, alias, alias}}
	if err := checkYAMLGraph(root); err != nil {
		t.Fatalf("shared node: %v", err)
	}
	alias.Alias = root
	if err := checkYAMLGraph(root); err == nil || !strings.Contains(err.Error(), "cyclic") {
		t.Fatalf("back edge: %v", err)
	}
	deep := shared
	for range maxYAMLDepth {
		deep = &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{deep}}
	}
	if err := checkYAMLGraph(deep); err == nil || !strings.Contains(err.Error(), "nesting") {
		t.Fatalf("nesting limit: %v", err)
	}
}
