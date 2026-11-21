// Package protoschema shows how a protobuf adapter gives clix's format
// extension a format.Schema built from a message descriptor, in a module
// of its own, so that clix's root module never depends on protobuf.
//
// The adapter marshals each message once, with protojson's ordinary
// (sparse) output, and attaches the schema of its message type. format.Write
// then fills in the fields protojson left out, the same way for every
// output flag: --format json, --jq, --template and row templates all see
// one canonical document. Nothing here reads the output flags.
//
//	cmd.Run = func(ctx *clix.Context) error {
//		doc, err := protoschema.Marshal(protojson.MarshalOptions{}, resp.GetInstance())
//		if err != nil {
//			return err
//		}
//		return format.Write(ctx, doc)
//	}
package protoschema

import (
	"encoding/json"

	"github.com/SCKelemen/clix/v2/ext/format"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Marshal returns m as a format.Document: m marshaled once with o, and
// the schema of its message type.
func Marshal(o protojson.MarshalOptions, m proto.Message) (format.Document, error) {
	b, err := o.Marshal(m)
	if err != nil {
		return format.Document{}, err
	}
	return format.Document{Value: json.RawMessage(b), Schema: Schema(m.ProtoReflect().Descriptor(), o)}, nil
}

// MarshalList returns messages, all of type M, as one format.Document: a
// list of their protojson output, each marshaled once with o, and the
// schema of M, which applies to each element. An empty list has the
// schema too, so --json and row templates can check field names.
func MarshalList[M proto.Message](o protojson.MarshalOptions, messages []M) (format.Document, error) {
	list := make([]json.RawMessage, len(messages))
	for i, m := range messages {
		b, err := o.Marshal(m)
		if err != nil {
			return format.Document{}, err
		}
		list[i] = b
	}
	var zero M // generated messages describe themselves through a nil pointer
	return format.Document{Value: list, Schema: Schema(zero.ProtoReflect().Descriptor(), o)}, nil
}

// Schema returns the fields of md's messages as protojson writes them
// with o: their JSON names (or proto names with o.UseProtoNames), in
// declaration order, with these values for a field the output leaves
// out:
//
//   - A field with presence (a message, a proto3 optional or proto2
//     field, or a member of a oneof) is null: it was not set. Unlike
//     protojson's EmitUnpopulated, which leaves unset oneof members out,
//     the schema lists them, so templates and --json can name them.
//   - A repeated field is [] and a map is {}.
//   - Any other field has its type's default as protojson writes it: 0,
//     "0" for 64-bit integers (which protojson quotes), "", false, and
//     the name of an enum's default value (its number with
//     o.UseEnumNumbers).
//
// A message field, or a repeated one, declares the fields of its message
// as nested fields, except for the well-known types that protojson
// writes in a form of their own (Timestamp as a string, Struct as any
// object, and so on) and a message type that contains itself, below its
// first level.
func Schema(md protoreflect.MessageDescriptor, o protojson.MarshalOptions) format.Fields {
	return messageFields(md, o, map[protoreflect.FullName]bool{})
}

func messageFields(md protoreflect.MessageDescriptor, o protojson.MarshalOptions, open map[protoreflect.FullName]bool) format.Fields {
	open[md.FullName()] = true
	defer delete(open, md.FullName())
	fds := md.Fields()
	fields := make(format.Fields, 0, fds.Len())
	for i := range fds.Len() {
		fd := fds.Get(i)
		f := format.Field{Name: fd.JSONName()}
		if o.UseProtoNames {
			f.Name = fd.TextName()
		}
		switch {
		case fd.IsMap():
			f.Zero = map[string]any{}
		case fd.IsList():
			f.Zero = []any{}
			f.Fields = nested(fd, o, open)
		case fd.HasPresence():
			f.Nullable = true
			f.Fields = nested(fd, o, open)
		default:
			f.Zero = zero(fd, o)
		}
		fields = append(fields, f)
	}
	return fields
}

// nested returns the fields of fd's message, or nil when fd is not a
// message, its message has a JSON form of its own, or it is already
// being described (a recursive type).
func nested(fd protoreflect.FieldDescriptor, o protojson.MarshalOptions, open map[protoreflect.FullName]bool) []format.Field {
	md := fd.Message()
	if md == nil || wellKnown[md.FullName()] || open[md.FullName()] {
		return nil
	}
	return messageFields(md, o, open)
}

// zero returns the value protojson writes for a field without presence
// that holds its default.
func zero(fd protoreflect.FieldDescriptor, o protojson.MarshalOptions) any {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return false
	case protoreflect.StringKind, protoreflect.BytesKind:
		return ""
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "0"
	case protoreflect.EnumKind:
		ed := fd.Enum()
		if ed.FullName() == "google.protobuf.NullValue" {
			return nil
		}
		n := fd.Default().Enum()
		if v := ed.Values().ByNumber(n); v != nil && !o.UseEnumNumbers {
			return string(v.Name())
		}
		return int32(n)
	}
	return 0 // 32-bit integers, float and double
}

// wellKnown are the message types protojson writes in a JSON form of
// their own rather than as an object of their fields.
// https://protobuf.dev/programming-guides/json/
var wellKnown = map[protoreflect.FullName]bool{
	"google.protobuf.Any":         true,
	"google.protobuf.Duration":    true,
	"google.protobuf.FieldMask":   true,
	"google.protobuf.ListValue":   true,
	"google.protobuf.Struct":      true,
	"google.protobuf.Timestamp":   true,
	"google.protobuf.Value":       true,
	"google.protobuf.BoolValue":   true,
	"google.protobuf.BytesValue":  true,
	"google.protobuf.DoubleValue": true,
	"google.protobuf.FloatValue":  true,
	"google.protobuf.Int32Value":  true,
	"google.protobuf.Int64Value":  true,
	"google.protobuf.StringValue": true,
	"google.protobuf.UInt32Value": true,
	"google.protobuf.UInt64Value": true,
}
