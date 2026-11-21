// Package protoformat adapts protobuf messages to clix's format extension.
package protoformat

import (
	"encoding/json"
	"errors"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Write renders a single protobuf message using the command's format flags.
func Write(ctx *clix.Context, message proto.Message) error {
	if ctx == nil || ctx.App == nil {
		return errors.New("protoformat.Write: no running command")
	}
	opts, err := MarshalOptions(ctx.App, protojson.MarshalOptions{})
	if err != nil {
		return err
	}
	data, err := opts.Marshal(message)
	if err != nil {
		return err
	}
	return format.Write(ctx, json.RawMessage(data))
}

// MarshalOptions returns the protobuf JSON options for the running command.
// Use the returned marshaler for each message in a list or a composite document,
// then pass the resulting json.RawMessage values to format.Write.
//
// Plain output uses base unchanged. Expressions include implicit scalar defaults
// and empty collections. Explicit --json selection also exposes unset fields that
// protobuf JSON can represent as null. The caller's resolver and naming options
// apply in all three cases.
func MarshalOptions(app *clix.App, base protojson.MarshalOptions) (protojson.MarshalOptions, error) {
	fields, err := format.FieldsFor(app)
	if err != nil {
		return base, err
	}
	switch fields {
	case format.SelectableFields:
		base.EmitUnpopulated = true
	case format.DefaultFields:
		base.EmitUnpopulated = false
		base.EmitDefaultValues = true
	}
	return base, nil
}
