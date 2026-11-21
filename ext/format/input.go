package format

import "github.com/SCKelemen/clix/v2"

// InputFields describes which fields a schema-aware producer can supply to Write.
type InputFields uint8

const (
	// PopulatedFields uses the producer's normal representation.
	PopulatedFields InputFields = iota
	// DefaultFields includes fields with implicit defaults and preserves explicit absence.
	DefaultFields
	// SelectableFields also makes absent fields available where the schema permits it.
	SelectableFields
)

// FieldsFor returns the input requested by the active output flags after parsing.
// An explicit --json field selection takes precedence over an expression on its result.
func FieldsFor(app *clix.App) (InputFields, error) {
	s, err := readSettings(app)
	if err != nil {
		return PopulatedFields, err
	}
	if err := s.check(); err != nil {
		return PopulatedFields, err
	}
	switch {
	case s.json:
		return SelectableFields, nil
	case s.jq != "" || s.template != "" || s.rowTemplate != nil:
		return DefaultFields, nil
	default:
		return PopulatedFields, nil
	}
}
