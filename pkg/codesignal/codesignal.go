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

// Build analyzes input and produces a Report. Diagnostics/Summary counts
// come from validateFileChange and the per-file head-result handling below;
// Signals come from mapping each Head Finding to a rule-defined Signal and
// then classifying it (and any base-only signal) against Base via
// classifyFileSignals, which also computes Fingerprint and ID. Sorting and
// filtering Report.Signals and populating the lifecycle-count Summary
// fields are a later task's job.
func (b *Builder) Build(ctx context.Context, input Input) (*Report, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	diagnostics := make([]Diagnostic, 0, len(input.Diagnostics))
	diagnostics = append(diagnostics, input.Diagnostics...)

	var signals []Signal

	for _, fc := range input.Files {
		diagnostics = append(diagnostics, validateFileChange(fc)...)

		fileDiagnostics, fileSignals := processHeadResult(fc)
		diagnostics = append(diagnostics, fileDiagnostics...)
		signals = append(signals, classifyFileSignals(fc.Base != nil, fileSignals, extractBaseSignals(fc))...)
	}

	sortDiagnostics(diagnostics)

	report := &Report{
		SchemaVersion: "1",
		Scope:         input.Scope,
		Summary: Summary{
			FilesAnalyzed:        len(input.Files),
			FilesWithDiagnostics: countFilesWithDiagnostics(input.Files, diagnostics),
		},
		Signals:     signals,
		Diagnostics: diagnostics,
	}

	return report, nil
}

// processHeadResult derives diagnostics and signals from fc.Head. A nil
// Head on a "modified" or "added" file is itself a diagnostic (there is
// nothing to analyze); a nil Head otherwise (e.g. "removed") produces no
// head signals, leaving classifyFileSignals to turn any base-only findings
// into "resolved" signals. A non-nil Head with ParseStatus "ok" maps its
// "mutates_input" Findings to Signals; "syntax_errors" surfaces one
// diagnostic per issue; any other ParseStatus is an unsupported-status
// diagnostic.
func processHeadResult(fc FileChange) ([]Diagnostic, []Signal) {
	if fc.Head == nil {
		if fc.Status == "modified" || fc.Status == "added" {
			return []Diagnostic{{
				Path:    fc.Path,
				Kind:    "missing_head_result",
				Message: "file change status \"" + string(fc.Status) + "\" has no head analysis result",
			}}, nil
		}
		return nil, nil
	}

	switch fc.Head.ParseStatus {
	case "ok":
		var signals []Signal
		for _, finding := range fc.Head.Findings {
			if finding.Kind != "mutates_input" {
				continue
			}
			signals = append(signals, newHiddenInputMutationSignal(fc.Path, finding))
		}
		return nil, signals
	case "syntax_errors":
		diagnostics := make([]Diagnostic, 0, len(fc.Head.SyntaxErrors))
		for _, issue := range fc.Head.SyntaxErrors {
			location := issue.Location
			diagnostics = append(diagnostics, Diagnostic{
				Path:     fc.Path,
				Kind:     "syntax_errors",
				Location: &location,
				Message:  "head analysis found a syntax issue of kind \"" + issue.Kind + "\"",
			})
		}
		return diagnostics, nil
	default:
		return []Diagnostic{{
			Path:    fc.Path,
			Kind:    "unsupported_parse_status",
			Message: "head analysis result has unsupported parse status \"" + string(fc.Head.ParseStatus) + "\"",
		}}, nil
	}
}

// extractBaseSignals maps fc.Base.Findings the same way processHeadResult
// maps fc.Head.Findings, but never emits diagnostics -- Base is reference
// data for lifecycle comparison only.
func extractBaseSignals(fc FileChange) []Signal {
	if fc.Base == nil || fc.Base.ParseStatus != "ok" {
		return nil
	}
	var signals []Signal
	for _, finding := range fc.Base.Findings {
		if finding.Kind != "mutates_input" {
			continue
		}
		signals = append(signals, newHiddenInputMutationSignal(fc.Path, finding))
	}
	return signals
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
