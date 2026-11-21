package protoformat_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
	"github.com/SCKelemen/clix/v2/ext/format/protoformat"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/typepb"
)

func TestOutputFieldsFollowTheParsedCommand(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PROBE_FORMAT", "")
	out := new(bytes.Buffer)
	app := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(new(bytes.Buffer)))
	app.AddExtension(format.Extension{RowTemplates: true, Projections: true})
	app.Root.AddCommand(clix.NewCommand("show", clix.WithCommandRun(func(ctx *clix.Context) error {
		return protoformat.Write(ctx, &typepb.Type{Name: "example.Record"})
	})))
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"ordinary JSON is sparse", []string{"--format", "json"}, "{\n  \"name\": \"example.Record\"\n}\n"},
		{"jq sees implicit defaults", []string{"--jq", `.edition == "" and .fields == [] and (has("sourceContext") | not)`}, "true\n"},
		{"document template sees empty collections", []string{"--template", `{{len .fields}}|{{printf "%q" .edition}}`}, "0|\"\""},
		{"row template sees default values", []string{"--format", `{{.name}}|{{printf "%q" .edition}}`}, "example.Record|\"\"\n"},
		{"explicit selection includes unset message", []string{"--json", "sourceContext", "--jq", `has("sourceContext") and .sourceContext == null`}, "true\n"},
		{"projection uses populated input", []string{"--format", "value(name)"}, "example.Record\n"},
		{"plain after expression is still sparse", []string{"--format", "json"}, "{\n  \"name\": \"example.Record\"\n}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out.Reset()
			if err := app.Run(t.Context(), append([]string{"show"}, tc.args...)); err != nil {
				t.Fatal(err)
			}
			if got := out.String(); got != tc.want {
				t.Errorf("output %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCompositeOutputKeepsProtoNamesAndResolver(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out := new(bytes.Buffer)
	app := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(new(bytes.Buffer)))
	app.AddExtension(format.Extension{RowTemplates: true})

	registry := new(protoregistry.Types)
	if err := registry.RegisterMessage((&typepb.Field{}).ProtoReflect().Type()); err != nil {
		t.Fatal(err)
	}
	field, err := anypb.New(&typepb.Field{Name: "amount"})
	if err != nil {
		t.Fatal(err)
	}
	base := protojson.MarshalOptions{Resolver: registry, UseProtoNames: true, UseEnumNumbers: true, EmitUnpopulated: true}
	app.Root.AddCommand(clix.NewCommand("list", clix.WithCommandRun(func(ctx *clix.Context) error {
		opts, err := protoformat.MarshalOptions(ctx.App, base)
		if err != nil {
			return err
		}
		if opts.Resolver != registry {
			t.Fatal("the custom Any resolver was replaced")
		}
		primary, err := opts.Marshal(&typepb.Type{Name: "Record"})
		if err != nil {
			return err
		}
		nested, err := opts.Marshal(field)
		if err != nil {
			return err
		}
		row := map[string]json.RawMessage{"type": primary, "field": nested}
		return format.Write(ctx, []map[string]json.RawMessage{row})
	})))
	for _, args := range [][]string{
		{"--jq", `.[0].type.edition == "" and (.[0].type | has("source_context") | not) and .[0].field.json_name == "" and .[0].field.kind == 0`},
		{"--format", `{{.type.name}}:{{.field.name}}:{{.field.packed}}`},
	} {
		out.Reset()
		if err := app.Run(t.Context(), append([]string{"list"}, args...)); err != nil {
			t.Fatal(err)
		}
		want := "true"
		if args[0] == "--format" {
			want = "Record:amount:false"
		}
		if got := strings.TrimSpace(out.String()); got != want {
			t.Fatalf("%s: got %q, want %q", args[0], got, want)
		}
	}
}

// A conflicting flag combination must fail before a command using MarshalOptions does work.
func TestMarshalOptionsRejectsConflictsBeforePreparingData(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	app := clix.NewApp("probe", clix.WithAppOut(new(bytes.Buffer)), clix.WithAppErr(new(bytes.Buffer)))
	app.AddExtension(format.Extension{RowTemplates: true})
	prepared := false
	app.Root.AddCommand(clix.NewCommand("show", clix.WithCommandRun(func(ctx *clix.Context) error {
		_, err := protoformat.MarshalOptions(ctx.App, protojson.MarshalOptions{})
		if err == nil {
			prepared = true
		}
		return err
	})))
	err := app.Run(t.Context(), []string{"show", "--jq", ".", "--format", "{{.name}}"})
	if err == nil || prepared {
		t.Fatalf("conflicting flags: prepared %t, error %v", prepared, err)
	}
}
