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
// those fire unconditionally for every file, since they're about data
// integrity, not analysis state. Signals come from mapping each Head
// Finding to a rule-defined Signal and then classifying it (and any
// base-only signal) against Base via classifyFileSignals, which also
// computes Fingerprint and ID -- but only when
// eligibleForLifecycleClassification(fc) holds: a file whose Head state
// already produced its own diagnostic (missing head, syntax errors,
// unsupported parse status) contributes zero signals rather than letting
// classifyFileSignals misread the empty head-signal set as "every
// base-derived finding was resolved". Changed is computed per file against
// its ChangedRanges (markChanged), lifecycle counts in Summary are tallied
// over the full unfiltered signal set, and Report.Signals is filtered by
// Options.IncludeResolved and sorted (sortSignals) before ActiveSignals is
// set to its final length.
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

		rangeDiagnostics, validRanges := validateChangedRanges(fc)
		diagnostics = append(diagnostics, rangeDiagnostics...)

		if eligibleForLifecycleClassification(fc) {
			fileClassifiedSignals := classifyFileSignals(baseUsableForLifecycle(fc), fileSignals, extractBaseSignals(fc))
			markChanged(fileClassifiedSignals, validRanges)
			signals = append(signals, fileClassifiedSignals...)
		}
	}

	sortDiagnostics(diagnostics)

	summary := Summary{
		FilesAnalyzed:        len(input.Files),
		FilesWithDiagnostics: countFilesWithDiagnostics(input.Files, diagnostics),
	}
	for _, sig := range signals {
		switch sig.Lifecycle {
		case "introduced":
			summary.IntroducedSignals++
		case "existing":
			summary.ExistingSignals++
		case "resolved":
			summary.ResolvedSignals++
		}
	}

	if !b.options.IncludeResolved {
		filtered := signals[:0]
		for _, sig := range signals {
			if sig.Lifecycle == "resolved" {
				continue
			}
			filtered = append(filtered, sig)
		}
		signals = filtered
	}

	sortSignals(signals)
	summary.ActiveSignals = len(signals)

	report := &Report{
		SchemaVersion: "1",
		Scope:         input.Scope,
		Summary:       summary,
		Signals:       signals,
		Diagnostics:   diagnostics,
	}

	return report, nil
}

// processHeadResult derives diagnostics and signals from fc.Head. A nil
// Head on a "modified" or "added" file is itself a diagnostic (there is
// nothing to analyze); a nil Head otherwise (e.g. "removed") produces no
// head signals -- Build only proceeds to classifyFileSignals for this case
// via eligibleForLifecycleClassification's removed-file rule, so a
// removed file's base-only findings still become "resolved" signals. A
// non-nil Head with ParseStatus "ok" maps its "mutates_input" Findings to
// Signals; "syntax_errors" surfaces one diagnostic per issue; any other
// ParseStatus is an unsupported-status diagnostic. In the syntax-errors and
// unsupported-status cases (and the missing-head case above),
// eligibleForLifecycleClassification is false, so Build skips
// classifyFileSignals entirely for this file rather than letting an empty
// head-signal set be misread as "every base finding was resolved".
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
// data for lifecycle comparison only. A Base that fails
// baseUsableForLifecycle (nil, or its Path disagreeing with fc.Path) is not
// trusted here even though validateFileChange already reports that
// mismatch separately as an invalid_file_change diagnostic.
func extractBaseSignals(fc FileChange) []Signal {
	if !baseUsableForLifecycle(fc) || fc.Base.ParseStatus != "ok" {
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

// baseUsableForLifecycle reports whether fc.Base can be trusted as a
// lifecycle baseline: present, and not already flagged by
// validateFileChange as having a Path that disagrees with fc.Path --
// mismatched Base data must not silently influence lifecycle
// classification just because validateFileChange already surfaced it as
// an invalid_file_change diagnostic.
func baseUsableForLifecycle(fc FileChange) bool {
	if fc.Base == nil {
		return false
	}
	return fc.Base.Path == "" || fc.Base.Path == fc.Path
}

// eligibleForLifecycleClassification reports whether fc's Head state is
// trustworthy enough to run signal emission/lifecycle classification at
// all: a clean head parse, or an explicitly removed file with no head
// (Story 4's removed-file rule). Every other Head state -- a missing
// head on a modified/added file, syntax errors, or an unsupported parse
// status -- already produced its own diagnostic in processHeadResult and
// must not additionally report Base-only findings as "resolved" just
// because the current file couldn't be analyzed.
func eligibleForLifecycleClassification(fc FileChange) bool {
	if fc.Head != nil {
		return fc.Head.ParseStatus == "ok"
	}
	return fc.Status == "removed"
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
