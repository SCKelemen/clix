package protoschema_test

import (
	"bytes"
	"strings"
	"testing"

	"clix/examples/protoschema"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/typepb"
)

// app builds "probe show", which prints doc with format.Write.
func app(t *testing.T, doc func() (format.Document, error)) (*clix.App, *bytes.Buffer) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PROBE_FORMAT", "")
	out := new(bytes.Buffer)
	a := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(new(bytes.Buffer)), clix.WithAppInteractive(clix.InteractiveNever))
	a.AddExtension(format.Extension{RowTemplates: true, Projections: true})
	a.Root.AddCommand(clix.NewCommand("show", format.WithListFlags(), clix.WithCommandRun(func(ctx *clix.Context) error {
		d, err := doc()
		if err != nil {
			return err
		}
		return format.Write(ctx, d)
	})))
	return a, out
}

func run(t *testing.T, a *clix.App, out *bytes.Buffer, args ...string) string {
	t.Helper()
	out.Reset()
	if err := a.Run(t.Context(), append([]string{"show"}, args...)); err != nil {
		t.Fatalf("%q: %v", args, err)
	}
	return out.String()
}

// The canonical document of typepb.Type{Name: "example.Record"}: protojson
// writes only the name; the schema fills in the rest.
const record = `{
  "name": "example.Record",
  "fields": [],
  "oneofs": [],
  "options": [],
  "sourceContext": null,
  "syntax": "SYNTAX_PROTO2",
  "edition": ""
}
`

// Every output flag sees the same document for a sparse message (the
// opposite of an adapter that marshals differently for each flag).
func TestEveryFlagSeesOneDocument(t *testing.T) {
	a, out := app(t, func() (format.Document, error) {
		return protoschema.Marshal(protojson.MarshalOptions{}, &typepb.Type{Name: "example.Record"})
	})
	plain := run(t, a, out, "--format", "json")
	if plain != record {
		t.Fatalf("--format json:\n%s\nwant:\n%s", plain, record)
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--jq", "."}, plain},
		{[]string{"--json", "name,fields,oneofs,options,sourceContext,syntax,edition"}, plain},
		{[]string{"--jq", `.edition == "" and .fields == [] and .sourceContext == null`}, "true\n"},
		{[]string{"--template", `{{len .fields}}|{{printf "%q" .edition}}|{{.syntax}}`}, "0|\"\"|SYNTAX_PROTO2"},
		{[]string{"--format", `{{.name}}|{{printf "%q" .edition}}|{{.syntax}}`}, "example.Record|\"\"|SYNTAX_PROTO2\n"},
		{[]string{"--json", "sourceContext", "--jq", `has("sourceContext") and .sourceContext == null`}, "true\n"},
		{[]string{"--format", "value(name,syntax)"}, "example.Record\tSYNTAX_PROTO2\n"},
	} {
		if got := run(t, a, out, tc.args...); got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.args, got, tc.want)
		}
	}
}

// A list of messages is marshaled once per message; the schema applies
// to every element and to the messages nested in them.
func TestListAndNestedMessages(t *testing.T) {
	types := []*typepb.Type{
		{Name: "a.A", Fields: []*typepb.Field{{Name: "id", Number: 1, Kind: typepb.Field_TYPE_INT64}, {Name: "note"}}},
		{Name: "b.B", SourceContext: nil},
	}
	a, out := app(t, func() (format.Document, error) {
		return protoschema.MarshalList(protojson.MarshalOptions{}, types)
	})
	if got, want := run(t, a, out, "--format", `{{.name}}:{{range .fields}} {{.name}}/{{.number}}/{{.kind}}/{{.packed}}{{end}}`), "a.A: id/1/TYPE_INT64/false note/0/TYPE_UNKNOWN/false\nb.B:\n"; got != want {
		t.Errorf("rows: got %q, want %q", got, want)
	}
	if got, want := run(t, a, out, "--filter", "fields.name:note", "--jq", ".[].name"), "a.A\n"; got != want {
		t.Errorf("--filter: got %q, want %q", got, want)
	}

	// No messages: the schema still knows the fields.
	empty, out := app(t, func() (format.Document, error) {
		return protoschema.MarshalList(protojson.MarshalOptions{}, []*typepb.Type{})
	})
	if got := run(t, empty, out, "--json", "sourceContext.fileName"); got != "[]\n" {
		t.Errorf("empty --json: got %q", got)
	}
	out.Reset()
	err := empty.Run(t.Context(), []string{"show", "--format", "{{.nmae}}"})
	if err == nil || clix.ExitCode(err) != 2 || !strings.Contains(err.Error(), `unknown field for --format: "nmae"`) {
		t.Errorf("unknown field in a row template: %v", err)
	}
	err = empty.Run(t.Context(), []string{"show", "--json", "sourceContext.fileNmae"})
	if err == nil || clix.ExitCode(err) != 2 || !strings.Contains(err.Error(), "sourceContext.fileName") {
		t.Errorf("unknown nested field for --json: %v", err)
	}
}

// The schema follows the marshal options that change names and values.
func TestMarshalOptions(t *testing.T) {
	a, out := app(t, func() (format.Document, error) {
		return protoschema.Marshal(protojson.MarshalOptions{UseProtoNames: true, UseEnumNumbers: true}, &typepb.Type{Name: "x"})
	})
	if got, want := run(t, a, out, "--jq", "[.source_context, .syntax]"), "[\n  null,\n  0\n]\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Presence, 64-bit integers, recursive types and well-known types.
func TestSchemaOfDescriptor(t *testing.T) {
	fields := protoschema.Schema((&descriptorpb.DescriptorProto{}).ProtoReflect().Descriptor(), protojson.MarshalOptions{})
	byName := map[string]format.Field{}
	for _, f := range fields {
		byName[f.Name] = f
	}
	// proto2: every singular field has presence.
	if f := byName["name"]; !f.Nullable {
		t.Errorf("name = %+v, want nullable", f)
	}
	// nestedType is a list of DescriptorProto: described once, not again
	// below itself.
	if f := byName["nestedType"]; f.Fields != nil {
		t.Errorf("nestedType has nested fields %v", f.Fields)
	}
	if f := byName["field"]; len(f.Fields) == 0 {
		t.Error("field has no nested fields")
	}
	// Any is opaque.
	typeFields := protoschema.Schema((&typepb.Option{}).ProtoReflect().Descriptor(), protojson.MarshalOptions{})
	if f := typeFields[1]; f.Name != "value" || !f.Nullable || f.Fields != nil {
		t.Errorf("Option.value = %+v, want an opaque nullable Any", f)
	}
	// proto3 scalars without presence: protojson quotes 64-bit integers.
	durationFields := protoschema.Schema((&durationpb.Duration{}).ProtoReflect().Descriptor(), protojson.MarshalOptions{})
	if got := durationFields; len(got) != 2 || got[0].Zero != "0" || got[1].Zero != 0 || got[0].Nullable {
		t.Errorf("Duration fields = %+v, want seconds \"0\" and nanos 0", got)
	}
}
