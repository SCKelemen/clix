package clix

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	flagsFileFlagName = "flags-file"

	// maxFlagsFileDepth is how deeply flags files may name other flags
	// files.
	maxFlagsFileDepth = 16

	// maxFlagsFiles is how many flags files one Run may read, so files
	// that name each other several times cannot multiply without end.
	maxFlagsFiles = 128

	// maxFlagsFileValues is how many flag values one Run may take from
	// flags files, so YAML aliases cannot multiply a small file.
	maxFlagsFileValues = 10000
)

// flagsFileValue is the Value of the --flags-file flag that
// EnableFlagsFile registers. Its type tells that flag apart from an app
// flag of the same name, which wins and is then an ordinary flag.
type flagsFileValue struct{ paths []string }

// Set records a flags file named on the command line.
func (v *flagsFileValue) Set(s string) error {
	v.paths = append(v.paths, s)
	return nil
}

// String returns the flags files named on the command line, joined like a
// StringSliceValue.
func (v *flagsFileValue) String() string { return joinList(v.paths) }

// Reset lets one App run more than once (see FlagSet.Var).
func (v *flagsFileValue) Reset() { v.paths = nil }

// isFlagsFileFlag reports whether f is the --flags-file flag that
// EnableFlagsFile registered.
func isFlagsFileFlag(f *Flag) bool {
	_, ok := f.Value.(*flagsFileValue)
	return ok
}

// EnableFlagsFile registers the global --flags-file flag, as gcloud has
// (see "gcloud topic flags-file"). --flags-file=FILE reads flags from a
// YAML or JSON file, where values need no shell quoting, and inserts them
// on the command line where --flags-file is written:
//
//	# deploy.yaml
//	--region: us-east1
//	--labels: [team=web, "note=a, b"]   # a list repeats the flag
//	--force:                            # no value: a switch is on
//	--flags-file: common.yaml           # other flags files, relative to this one
//
//	myapp deploy --flags-file deploy.yaml --region eu-west1
//
// The file is a mapping of long flag names, with or without the leading
// "--", to values, or a list of such mappings, which may name a flag
// again. A value is a string, number or boolean, used as the text written
// in the file ("0755" stays 0755), as if typed after --name=; a list
// repeats the flag once per item, as writing it several times on the
// command line does; no value (null) is the flag alone, which turns a
// switch on. "--flags-file" names more flags files (a list names
// several), read in place; relative paths are relative to the directory
// of the file that names them, and a file that names itself, directly or
// through others, is an error. "-" reads standard input, once per Run.
//
// Precedence follows gcloud: each --flags-file is replaced by the flags in
// its file, so the usual command-line rules apply. For a flag given more
// than once, the last occurrence wins (slice flags append), so a flag
// written after --flags-file overrides the file and one written before it
// is overridden. Values from the file are command-line values: they beat
// environment variables and the config file, count for FlagSet.Changed,
// report SourceCommandFlag (or SourceAppFlag), may set CommandLineOnly
// flags, and count as flags passed when Run decides whether to prompt for
// missing required flags, unless the flag is Ambient. --flags-file itself
// is Ambient, so a file that holds only Ambient flags still lets Run
// prompt. Values of flags with ValueFromFile may read files themselves
// ("--body: '@body.md'"), relative to the flags file's directory.
//
// Flags in the file are looked up where --flags-file is written, as a
// flag written there would be. A key that names no flag there is a usage
// error, even on the non-strict root, and so are a duplicate key, a value
// that is a mapping, a list of lists, and a value the flag rejects;
// errors name the file and the line. A Sensitive flag's value from a
// flags file never appears in an error. Each file is read up to
// App.MaxFileSize. YAML merge keys (<<) are not supported.
//
// The flag is read from the command line only (never the environment or
// the config file). An app flag named --flags-file wins over it, as app
// flags win over extensions'; it is then an ordinary flag. Calling
// EnableFlagsFile again does nothing.
func (a *App) EnableFlagsFile() {
	if a.flagsFile {
		return
	}
	a.flagsFile = true
	a.Flags().Var(VarOptions{
		FlagOptions: FlagOptions{
			Name:            flagsFileFlagName,
			Usage:           "Read flags from a YAML or JSON file, inserted where this flag is (- for standard input)",
			CommandLineOnly: true,
			Ambient:         true,
		},
		Value: &flagsFileValue{},
	})
}

// FlagsFileEnabled reports whether EnableFlagsFile was called.
func (a *App) FlagsFileEnabled() bool { return a.flagsFile }

// applyFlagsFileUse applies the --flags-file use u: it reads the file and
// applies its flags as if they were written in its place, where the flag
// sets in effect are sets (and late). next supplies the file name when it
// is the following argument.
func (r *fileReader) applyFlagsFileUse(u flagUse, sets, late []*FlagSet, next func() (string, bool)) error {
	path := u.value
	if !u.hasValue {
		v, ok := "", false
		if next != nil {
			v, ok = next()
		}
		if !ok {
			return usageErrorf("flag %s requires a value", u.name)
		}
		path = v
	}
	if path == "" {
		return usageErrorf("%s: name a file, or - for standard input", u.name)
	}
	if err := u.flag.setValue(path); err != nil {
		return usageErrorf("invalid value for %s: %w", u.flag.Name, err)
	}
	u.flag.markCLI()
	values := 0
	return r.flagsFile(path, sets, late, nil, &values)
}

// flagsFileFrame is a flags file being read, for finding cycles.
type flagsFileFrame struct {
	path string
	info os.FileInfo // nil for standard input
}

// flagsFile reads the flags file at path and applies its flags. stack
// holds the files that named it, outermost first; values counts the flag
// values taken from flags files so far.
func (r *fileReader) flagsFile(path string, sets, late []*FlagSet, stack []flagsFileFrame, values *int) error {
	shown := displayPath(path)
	if len(stack) >= maxFlagsFileDepth {
		return usageErrorf("--flags-file %s: flags files are nested more than %d deep", shown, maxFlagsFileDepth)
	}
	r.files++
	if r.files > maxFlagsFiles {
		return usageErrorf("--flags-file %s: more than %d flags files", shown, maxFlagsFiles)
	}
	data, info, err := r.read(path, "--flags-file")
	if err != nil {
		return &usageError{err: err}
	}
	for i, fr := range stack {
		if fr.info != nil && info != nil && os.SameFile(fr.info, info) {
			chain := make([]string, 0, len(stack)-i+1)
			for _, f := range stack[i:] {
				chain = append(chain, displayPath(f.path))
			}
			chain = append(chain, shown)
			return usageErrorf("--flags-file: %s names itself: %s", displayPath(fr.path), strings.Join(chain, " -> "))
		}
	}
	stack = append(stack, flagsFileFrame{path: path, info: info})

	entries, err := parseFlagsFile(data, values)
	if err != nil {
		return usageErrorf("flags file %s: %w", shown, err)
	}
	dir := ""
	if path != "-" {
		dir = filepath.Dir(path)
	}
	for _, e := range entries {
		flag := lookupArg("--"+e.name, sets...)
		if flag == nil {
			flag = lookupArg("--"+e.name, late...)
		}
		if flag == nil {
			return usageErrorf("flags file %s, line %d: unknown flag --%s", shown, e.line, e.name)
		}
		if isFlagsFileFlag(flag) {
			for _, v := range e.args {
				if v.null || v.text == "" {
					return usageErrorf("flags file %s, line %d: --%s needs a file name", shown, v.line, e.name)
				}
				if err := r.flagsFile(resolvePath(dir, v.text), sets, late, stack, values); err != nil {
					return err
				}
			}
			continue
		}
		for _, v := range e.args {
			var err error
			if v.null {
				err = r.setArg(flag, "--"+e.name, "", false, nil, path)
			} else {
				err = r.setArg(flag, "--"+e.name, v.text, true, nil, path)
			}
			if err != nil {
				return usageErrorf("flags file %s, line %d: %w", shown, v.line, err)
			}
		}
	}
	return nil
}

// flagsFileEntry is one key of a flags file: a flag with its values.
type flagsFileEntry struct {
	name string // the long name, without "--"
	line int
	args []flagsFileArg
}

// flagsFileArg is one value of a flag in a flags file.
type flagsFileArg struct {
	text string
	null bool // no value: the flag alone
	line int
}

// parseFlagsFile reads the entries of a flags file, in order: a mapping
// of flags to values, or a list of such mappings. values counts the flag
// values read so far, which may not exceed maxFlagsFileValues.
func parseFlagsFile(data []byte, values *int) ([]flagsFileEntry, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil // empty, or only comments
		}
		return nil, err
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("line %d: a flags file holds one YAML document", extra.Line)
	}
	n := &doc
	if n.Kind == yaml.DocumentNode {
		if len(n.Content) == 0 {
			return nil, nil
		}
		n = n.Content[0]
	}
	n = resolveAlias(n)
	switch {
	case n.Kind == yaml.ScalarNode && n.ShortTag() == "!!null":
		return nil, nil
	case n.Kind == yaml.MappingNode:
		return flagsFileMapping(n, values)
	case n.Kind == yaml.SequenceNode:
		var entries []flagsFileEntry
		for _, item := range n.Content {
			item = resolveAlias(item)
			if item.Kind != yaml.MappingNode {
				return nil, fmt.Errorf("line %d: each item of the list is a mapping of flags to values", item.Line)
			}
			more, err := flagsFileMapping(item, values)
			if err != nil {
				return nil, err
			}
			entries = append(entries, more...)
		}
		return entries, nil
	}
	return nil, fmt.Errorf("line %d: a flags file is a mapping of flags to values, or a list of such mappings", n.Line)
}

// flagsFileMapping reads the entries of one mapping of a flags file.
func flagsFileMapping(n *yaml.Node, values *int) ([]flagsFileEntry, error) {
	var entries []flagsFileEntry
	seen := make(map[string]int)
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, value := resolveAlias(n.Content[i]), n.Content[i+1]
		if key.ShortTag() == "!!merge" {
			return nil, fmt.Errorf("line %d: merge keys (<<) are not supported in a flags file", key.Line)
		}
		if key.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("line %d: keys must be flag names", key.Line)
		}
		name, err := flagsFileKey(key.Value)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", key.Line, err)
		}
		if first, dup := seen[name]; dup {
			return nil, fmt.Errorf("line %d: --%s is already given on line %d (use a list to give a flag more than once)", key.Line, name, first)
		}
		seen[name] = key.Line
		args, err := flagsFileArgs(name, value)
		if err != nil {
			return nil, err
		}
		*values += len(args)
		if *values > maxFlagsFileValues {
			return nil, fmt.Errorf("line %d: more than %d flag values", key.Line, maxFlagsFileValues)
		}
		entries = append(entries, flagsFileEntry{name: name, line: key.Line, args: args})
	}
	return entries, nil
}

// flagsFileKey returns the long flag name a flags file key names: the key
// itself, or the key without a leading "--".
func flagsFileKey(key string) (string, error) {
	name := strings.TrimPrefix(key, "--")
	if name == "" || strings.HasPrefix(name, "-") || strings.ContainsAny(name, "= \t\r\n") {
		return "", fmt.Errorf("%q is not a long flag name, such as --region", key)
	}
	return name, nil
}

// flagsFileArgs returns the values of the flag name, written as n in a
// flags file: a scalar is one value, a list one value per item.
func flagsFileArgs(name string, n *yaml.Node) ([]flagsFileArg, error) {
	n = resolveAlias(n)
	switch n.Kind {
	case yaml.ScalarNode:
		return []flagsFileArg{scalarArg(n)}, nil
	case yaml.SequenceNode:
		args := make([]flagsFileArg, 0, len(n.Content))
		for _, item := range n.Content {
			item = resolveAlias(item)
			if item.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("line %d: --%s: each item of a list is one value", item.Line, name)
			}
			args = append(args, scalarArg(item))
		}
		return args, nil
	}
	return nil, fmt.Errorf("line %d: --%s: a mapping is not a flag value", n.Line, name)
}

// scalarArg returns the value a scalar gives: its text as written, or no
// value for null.
func scalarArg(n *yaml.Node) flagsFileArg {
	if n.ShortTag() == "!!null" {
		return flagsFileArg{null: true, line: n.Line}
	}
	return flagsFileArg{text: n.Value, line: n.Line}
}
