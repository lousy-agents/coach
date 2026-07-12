// Package codesignal derives coaching signals from pkg/semantics analysis
// results across a set of changed files.
package codesignal

import "context"

// Options configures a Builder. It has no fields in v0.1; kept as a struct
// (rather than removed) so New's signature doesn't need to change once
// options exist.
type Options struct {
	IncludeResolved bool `json:"include_resolved"`
}

// Builder produces Reports from Input. It holds no mutable state after
// construction (options is copied in New and never written to again), so a
// *Builder is safe for concurrent Build calls without additional
// synchronization.
type Builder struct {
	options Options
}

// New constructs a Builder from options. options is copied, not aliased, so
// later mutation of the caller's Options value has no effect on the
// Builder. New cannot fail in v0.1 (no fields to validate yet); the error
// return is kept for API stability as validation is added later.
func New(options Options) (*Builder, error) {
	return &Builder{options: options}, nil
}

// Build analyzes input and produces a Report. In this task Build only
// validates FileChange identity and populates Diagnostics/Summary counts;
// rule/signal emission is added by later tasks on top of the per-file
// processing loop below.
func (b *Builder) Build(ctx context.Context, input Input) (*Report, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	diagnostics := make([]Diagnostic, 0, len(input.Diagnostics))
	diagnostics = append(diagnostics, input.Diagnostics...)

	for _, fc := range input.Files {
		diagnostics = append(diagnostics, validateFileChange(fc)...)
	}

	sortDiagnostics(diagnostics)

	report := &Report{
		SchemaVersion: "1",
		Scope:         input.Scope,
		Summary: Summary{
			FilesAnalyzed:        len(input.Files),
			FilesWithDiagnostics: countFilesWithDiagnostics(input.Files, diagnostics),
		},
		Diagnostics: diagnostics,
	}

	return report, nil
}

// validateFileChange checks that fc.Base/fc.Head, when present with a
// non-empty Path, agree with fc.Path -- the canonical file identity used
// for signals/diagnostics/fingerprints in later tasks.
func validateFileChange(fc FileChange) []Diagnostic {
	var diagnostics []Diagnostic

	if fc.Base != nil && fc.Base.Path != "" && fc.Base.Path != fc.Path {
		diagnostics = append(diagnostics, Diagnostic{
			Path:    fc.Path,
			Kind:    "invalid_file_change",
			Message: "base result path \"" + fc.Base.Path + "\" does not match file change path \"" + fc.Path + "\"",
		})
	}
	if fc.Head != nil && fc.Head.Path != "" && fc.Head.Path != fc.Path {
		diagnostics = append(diagnostics, Diagnostic{
			Path:    fc.Path,
			Kind:    "invalid_file_change",
			Message: "head result path \"" + fc.Head.Path + "\" does not match file change path \"" + fc.Path + "\"",
		})
	}

	return diagnostics
}

// countFilesWithDiagnostics counts the distinct FileChange.Path values in
// files that have at least one diagnostic attributed to them.
func countFilesWithDiagnostics(files []FileChange, diagnostics []Diagnostic) int {
	withDiagnostics := make(map[string]bool, len(diagnostics))
	for _, d := range diagnostics {
		withDiagnostics[d.Path] = true
	}

	count := 0
	seen := make(map[string]bool, len(files))
	for _, fc := range files {
		if seen[fc.Path] {
			continue
		}
		seen[fc.Path] = true
		if withDiagnostics[fc.Path] {
			count++
		}
	}

	return count
}
