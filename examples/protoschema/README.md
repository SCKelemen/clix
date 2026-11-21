# Protobuf schema adapter

This example shows how a protobuf adapter plugs into clix's format extension without clix depending on protobuf. It is a module of its own: `google.golang.org/protobuf` appears in this directory's `go.mod`, never in clix's root module.

protojson's ordinary output leaves out fields that hold their defaults, so `{{.edition}}` in a row template would be an unknown field. Rather than marshaling differently for each output flag, the adapter marshals each message once and attaches a [`format.Schema`](../../docs/output.md#schemas-and-missing-fields) built from the message descriptor. `format.Write` fills in the missing fields once, so `--format json`, `--jq`, `--template`, row templates, `--json` and gcloud projections all see the same document.

[`protoschema.go`](protoschema.go) has the adapter:

- `Schema(md, opts)` walks a `protoreflect.MessageDescriptor` and returns `format.Fields`: JSON names (or proto names with `UseProtoNames`), `null` for fields with presence (messages, `optional` and proto2 fields, oneof members), `[]` and `{}` for repeated fields and maps, and protojson's defaults for the rest (`"0"` for 64-bit integers, an enum's default name). Message fields declare their message's fields as nested fields, except the well-known types that have a JSON form of their own and recursive types below their first level.
- `Marshal(opts, m)` returns a `format.Document` for one message, and `MarshalList(opts, messages)` returns one for a list, with the schema applied to every element.

A command prints a message with:

```go
// Abridged: needs google.golang.org/protobuf, which clix's root module does not have.
doc, err := protoschema.Marshal(protojson.MarshalOptions{}, instance)
if err != nil {
        return err
}
return format.Write(ctx, doc)
```

[`protoschema_test.go`](protoschema_test.go) checks that every output flag sees the same document for a sparse `google.protobuf.Type`, that lists and nested messages are filled, that an empty list still validates field names, and that the schema follows `UseProtoNames` and `UseEnumNumbers`.

Run the tests from this directory:

```bash
go test ./...
```
