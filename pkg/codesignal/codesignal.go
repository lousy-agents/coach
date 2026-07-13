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

// Build analyzes input and produces a Report.
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

// processHeadResult derives diagnostics and signals from fc.Head.
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

// extractBaseSignals maps fc.Base.Findings to signals for lifecycle
// comparison without emitting diagnostics.
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
// lifecycle baseline.
func baseUsableForLifecycle(fc FileChange) bool {
	if fc.Base == nil {
		return false
	}
	return fc.Base.Path == "" || fc.Base.Path == fc.Path
}

// eligibleForLifecycleClassification reports whether fc's Head state is
// clean enough to run lifecycle classification.
func eligibleForLifecycleClassification(fc FileChange) bool {
	if fc.Head != nil {
		return fc.Head.ParseStatus == "ok"
	}
	return fc.Status == "removed"
}

// validateFileChange checks that fc.Base/fc.Head, when present, agree
// with fc.Path.
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
