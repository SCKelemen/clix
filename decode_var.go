package clix

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// DecodeVarOptions configures a flag whose value names a YAML or JSON file
// that is strictly decoded into a Go value, like gcloud's --*-from-file
// flags and kubectl's -f. This struct implements FlagOption, so it can be
// used alongside functional options.
//
// Example:
//
//	type Spec struct {
//		Image    string            `json:"image"`
//		Replicas int               `json:"replicas"`
//		Env      map[string]string `json:"env"`
//	}
//	spec := Spec{Replicas: 1} // fields the file leaves out keep these values
//	cmd.Flags.DecodeVar(clix.DecodeVarOptions{
//		FlagOptions: clix.FlagOptions{Name: "spec", Short: "f", Usage: "Deployment spec (YAML or JSON file, - for stdin)", Required: true},
//		Value:       &spec,
//	})
//	// myapp deploy -f spec.yaml
//	// cat spec.json | myapp deploy -f -
type DecodeVarOptions struct {
	FlagOptions
	// Value is a non-nil pointer to the variable the file is decoded
	// into, usually a struct.
	Value any
}

// merge applies the non-zero fields of v on top of o.
func (o *DecodeVarOptions) merge(v DecodeVarOptions) {
	v.ApplyFlag(&o.FlagOptions)
	if v.Value != nil {
		o.Value = v.Value
	}
}

// DecodeVar registers a flag whose value names a YAML or JSON file, which
// is decoded into the variable Value points to with the same strict rules
// as UnmarshalStrict: unknown fields, duplicate keys and values of the
// wrong type are a usage error that lists every problem with its path
// (a *StrictError), names match exactly, and scalars keep the text
// written in the file where the field is text. Fields the file leaves
// out keep their values, so the variable's contents before Run are the
// defaults. When the file has a problem the variable is not changed.
// Accepts either a DecodeVarOptions struct (primary API) or functional
// options with WithDecodeValue.
//
// The flag's value is a file name: "spec.yaml", or "-" for standard
// input. A leading "@" is allowed and ignored, so "@spec.yaml" and "@-"
// work as for a ValueFromFile flag ("@@x" names the file "@x"). Files and
// standard input are read up to App.MaxFileSize, and standard input once
// per Run. In a flags file (see App.EnableFlagsFile), a relative name is
// relative to the flags file's directory.
//
// The value may come from every source: the command line, an environment
// variable (which names a file), the config file, or a prompt for a
// Required flag (which asks for a file name). In the config file, the
// flag's key may hold the document itself, as a mapping or a list, which
// is decoded as ConfigManager.Decode does; a key that holds a single
// value names a file. Repeating the flag decodes each file in turn into
// the same variable.
//
// For a Sensitive flag, errors name fields by path but never include a
// value, or a message that might quote one. Validate, if set, receives
// the file name (it is not called for a config section). String returns the name of the file last read, and
// Context getters return it too. Between Runs the variable is put back as
// it was with a shallow copy, so maps and pointers that the variable held
// before Run are shared with what Run decoded into them.
func (fs *FlagSet) DecodeVar(opts ...FlagOption) {
	var decodeOpts DecodeVarOptions
	for _, opt := range opts {
		switch v := opt.(type) {
		case nil:
			continue
		case DecodeVarOptions:
			decodeOpts.merge(v)
		case *DecodeVarOptions:
			if v != nil {
				decodeOpts.merge(*v)
			}
		case decodeValueOption:
			decodeOpts.Value = v.value
		default:
			opt.ApplyFlag(&decodeOpts.FlagOptions)
		}
	}
	if err := checkTarget(decodeOpts.Value); err != nil {
		panic(fmt.Sprintf("clix: DecodeVar --%s: Value must be a non-nil pointer, not %T", decodeOpts.Name, decodeOpts.Value))
	}
	value := &decodeValue{target: decodeOpts.Value, flag: "--" + decodeOpts.Name, sensitive: decodeOpts.Sensitive}
	fs.addFlag(decodeOpts.newFlag(value, ""))
}

// WithDecodeValue sets the variable a flag registered with DecodeVar
// decodes into: a non-nil pointer.
func WithDecodeValue(value any) FlagOption {
	return decodeValueOption{value: value}
}

type decodeValueOption struct{ value any }

func (o decodeValueOption) ApplyFlag(*FlagOptions) {}

// errNoFileName is the problem with an empty DecodeVar value.
var errNoFileName = errors.New("name a file, or - for standard input")

// decodeValue is the Value of a DecodeVar flag.
type decodeValue struct {
	target    any    // a non-nil pointer
	flag      string // "--name", for messages
	sensitive bool
	name      string // the file last read
}

// Set reads and decodes the file text names, with the default size limit
// and os.Stdin for "-". Run reads it through setFromFile instead.
func (v *decodeValue) Set(text string) error {
	return v.setFromFile(newFileReader(nil), text, "")
}

// String returns the name of the file last read.
func (v *decodeValue) String() string { return v.name }

// setFromFile reads the file text names, relative to dir when dir is not
// empty, through r, and decodes it into the target.
func (v *decodeValue) setFromFile(r *fileReader, text, dir string) error {
	path := strings.TrimPrefix(text, "@")
	if path == "" {
		return errNoFileName
	}
	path = resolvePath(dir, path)
	data, _, err := r.read(path, v.flag)
	if err != nil {
		return err
	}
	if err := decodeDocument(data, v.target, decodeOptions{source: displayPath(path), sensitive: v.sensitive}); err != nil {
		return err
	}
	v.name = path
	return nil
}

// decodeConfig decodes the config file's section under key into the
// target. It reports false when key holds a single value (a file name)
// or nothing.
func (v *decodeValue) decodeConfig(m *ConfigManager, key string) (bool, error) {
	if _, isValue := m.Get(key); isValue {
		return false, nil
	}
	c := newYAMLConverter(key)
	root, err := m.subtree(key, c)
	if err != nil || root == nil {
		return err != nil, err
	}
	if err := decodeTree(root, c, v.target, decodeOptions{source: m.source(), base: key, sensitive: v.sensitive}); err != nil {
		return true, err
	}
	v.name = ""
	return true, nil
}

// snapshot implements valueSnapshotter: the variable and the file name
// are put back as they are now.
func (v *decodeValue) snapshot() func() {
	target := reflect.ValueOf(v.target).Elem()
	saved := reflect.New(target.Type()).Elem()
	saved.Set(target)
	name := v.name
	return func() {
		target.Set(saved)
		v.name = name
	}
}
