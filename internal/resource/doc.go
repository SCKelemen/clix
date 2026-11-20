// Package resource is the part of the Google Cloud CLI's resource library
// (googlecloudsdk/core/resource) that clix's gcloud-style list flags and
// output formats need, ported to Go with the standard library and
// golang.org/x/text only.
//
// A resource is one element of a command's result, read from the JSON
// that clix.FormatData writes the way Python's json module reads it
// (see Decode): objects keep their keys in order, whole numbers are
// arbitrary-precision integers and other numbers are float64, exactly as
// gcloud sees the same JSON. On top of that model the package provides:
//
//   - Python's str() and repr() of values and its int() and float() of
//     strings (python.go), which gcloud's comparisons and sort keys use;
//   - gcloud's expression lexer (resource_lex.Lexer) and resource keys
//     such as abc.def[3].ghi (lexer.go, key.go);
//   - resource_property.Get (get.go), with its case conversions
//     (project_id finds projectId) and list rules;
//   - --sort-by (sort.go);
//   - NormalizeForSearch, the case and accent folding of gcloud's ":" and
//     "=" filter operators (normalize.go).
//
// Each function says which gcloud function it ports. Behavior that the
// port does not reproduce is an *UnsupportedError, never a different
// result; differential tests compare the port with the gcloud library
// itself when CLIX_GCLOUD_SDK names an installed Cloud SDK.
package resource
