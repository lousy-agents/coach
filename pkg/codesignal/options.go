package codesignal

type Options struct {
	IncludeResolved bool `json:"include_resolved"`
	Baseline        bool `json:"baseline"`

	// ProjectEnabled switches Build onto the schema-2 project-analysis
	// report path: SchemaVersion becomes "2" and the Report's project_*
	// fields become eligible to serialize. See Report's field block for
	// the byte-identity guarantee this default-false zero value preserves.
	ProjectEnabled bool `json:"project_enabled"`
}

// New constructs a Builder from options. options is copied, not aliased, so
// later mutation of the caller's Options value has no effect on the
// Builder. New cannot fail in v0.1 (no fields to validate yet); the error
// return is kept for API stability as validation is added later.
func New(options Options) (*Builder, error) {
	return &Builder{options: options}, nil
}
