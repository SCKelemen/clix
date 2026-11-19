package format

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/internal/projection"
	"github.com/SCKelemen/clix/v2/internal/resource"
	"github.com/SCKelemen/clix/v2/internal/tty"
)

// writeProjection prints data with a gcloud format expression (see
// Extension.Projections): the JSON output of data, read as gcloud reads
// resources, is projected and printed as gcloud's printer of that name
// prints it. json and yaml print the projected resources with clix's
// JSON and YAML encoders, as --format json and yaml do.
func writeProjection(out io.Writer, data any, f *projection.Format, opts clix.FormatOptions) error {
	var raw bytes.Buffer
	if err := clix.FormatData(&raw, data, clix.FormatJSON); err != nil {
		return err
	}
	doc, err := resource.Decode(raw.Bytes())
	if err != nil {
		return err
	}
	if f.NeedsColumns() {
		keys := defaultColumns(doc, opts.Columns)
		if len(keys) == 0 && !resource.Truthy(doc) {
			return nil // no resources: gcloud prints nothing
		}
		if len(keys) == 0 {
			return clix.UsageErrorf("--format %s needs a projection, such as %s(name), because this output has no fields", f.Name, f.Name)
		}
		if f, err = f.WithColumns(keys); err != nil {
			return projectionError(err)
		}
	}
	switch f.Name {
	case clix.FormatJSON, clix.FormatYAML:
		v, ok := f.Project(doc)
		if !ok && f.Name == clix.FormatYAML {
			return nil // gcloud prints no YAML document for no resources
		}
		b, err := projection.JSON(v)
		if err != nil {
			return err
		}
		return clix.FormatData(out, json.RawMessage(b), f.Name)
	}
	if err := f.Print(out, doc, projection.Options{UTF8: tty.UTF8()}); err != nil {
		return projectionError(err)
	}
	return nil
}

// projectionError names what clix does not print as --format's, and makes
// a missing projection a usage error.
func projectionError(err error) error {
	var unsupported *resource.UnsupportedError
	switch {
	case errors.As(err, &unsupported):
		return fmt.Errorf("%s is not supported in clix --format", unsupported.What)
	case errors.Is(err, projection.ErrNeedsProjection):
		return clix.UsageErrorf("--%w", err)
	}
	return err
}

// defaultColumns are the keys of table, csv and value without a
// projection: the command's FormatOptions.Columns when it gives them (a
// dotted path such as owner.login is a nested key), else every top-level
// field of the output's objects, in the order they first appear.
func defaultColumns(doc any, columns []string) []resource.Key {
	var keys []resource.Key
	if len(columns) > 0 {
		for _, c := range columns {
			key, err := resource.ParseKey(c)
			if err != nil || len(key) == 0 {
				key = resource.Key{{Name: c}}
			}
			keys = append(keys, key)
		}
		return keys
	}
	seen := map[string]bool{}
	add := func(o *resource.Object) {
		for _, k := range o.Keys() {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, resource.Key{{Name: k}})
			}
		}
	}
	switch v := doc.(type) {
	case *resource.Object:
		add(v)
	case []any:
		for _, e := range v {
			if o, ok := e.(*resource.Object); ok {
				add(o)
			}
		}
	}
	return keys
}
