package clix

import "testing"

func TestCommandMatchingAndVisibility(t *testing.T) {
	root := NewCommand("root")
	child := NewCommand("child")
	child.Aliases = []string{"c"}
	grand := NewCommand("grand")
	other := NewCommand("other")
	hidden := NewCommand("hidden")
	hidden.Hidden = true

	child.AddCommand(grand)
	root.AddCommand(child)
	root.AddCommand(other)
	root.AddCommand(hidden)

	matched, rest := root.match([]string{"c", "grand", "extra"})
	if matched != grand {
		t.Fatalf("expected grand command, got %v", matched)
	}
	if len(rest) != 1 || rest[0] != "extra" {
		t.Fatalf("unexpected remaining args: %v", rest)
	}

	if path := grand.Path(); path != "root child grand" {
		t.Fatalf("unexpected command path: %q", path)
	}

	if resolved := root.ResolvePath([]string{"child", "grand"}); resolved != grand {
		t.Fatalf("ResolvePath returned %v", resolved)
	}

	if resolved := root.ResolvePath([]string{"child", "missing"}); resolved != nil {
		t.Fatalf("expected nil for missing command, got %v", resolved)
	}

	visible := root.VisibleChildren()
	if len(visible) != 2 {
		t.Fatalf("expected 2 visible children, got %d", len(visible))
	}
	if visible[0] != child || visible[1] != other {
		t.Fatalf("unexpected visible order: %v", visible)
	}
}

func TestPrepareStaticCommandTree(t *testing.T) {
	app := NewApp("demo")
	app.Root = &Command{
		Name: "root",
		Children: []*Command{{
			Name: "child",
			Children: []*Command{{
				Name: "grand",
			}},
		}},
	}

	app.ensureRootPrepared()

	if app.Root.Flags == nil {
		t.Fatalf("expected root flags to be initialised")
	}
	if app.Root.Flags.lookup("help") == nil {
		t.Fatalf("expected root help flag to be registered")
	}

	if len(app.Root.Children) != 1 {
		t.Fatalf("expected 1 child command, got %d", len(app.Root.Children))
	}
	child := app.Root.Children[0]
	if child.parent != app.Root {
		t.Fatalf("expected child parent to be root")
	}
	if child.Flags == nil {
		t.Fatalf("expected child flags to be initialised")
	}
	if child.Flags.lookup("help") == nil {
		t.Fatalf("expected child help flag to be registered")
	}

	if len(child.Children) != 1 {
		t.Fatalf("expected 1 grandchild command, got %d", len(child.Children))
	}
	grand := child.Children[0]
	if grand.parent != child {
		t.Fatalf("expected grandchild parent to be child")
	}
	if grand.Flags == nil {
		t.Fatalf("expected grandchild flags to be initialised")
	}
	if grand.Flags.lookup("help") == nil {
		t.Fatalf("expected grandchild help flag to be registered")
	}
}
