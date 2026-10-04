package codesignal

import (
	"strings"
)

func projectLifecycleDiagnosticMessage(input Input) string {
	reasons := make([]string, 0, 2)
	if input.ProjectCoverage == nil {
		reasons = append(reasons, "head coverage unavailable")
	} else if !input.ProjectCoverage.Complete {
		reasons = append(reasons, "head coverage incomplete")
	}

	baseSideExpected := input.ProjectBaseAnalyzed || len(input.BaseProjectChanges) > 0
	if baseSideExpected {
		if !input.ProjectBaseAnalyzed || input.BaseProjectCoverage == nil {
			reasons = append(reasons, "base coverage unavailable")
		} else if !input.BaseProjectCoverage.Complete {
			reasons = append(reasons, "base coverage incomplete")
		}
	}
	if len(reasons) == 0 {
		return "project lifecycle is indeterminate"
	}
	return "project lifecycle is indeterminate: " + strings.Join(reasons, "; ")
}
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
		counts := findingCountsByKind(fc.Head.Findings)
		signals := signalsFromFindings(fc.Path, fc.Head.Findings, counts)
		signals = append(signals, signalsFromMetrics(fc.Path, fc.Head.Metrics)...)
		signals = append(signals, signalsFromImports(fc.Path, fc.Head.Language, fc.Head.Imports)...)
		signals = append(signals, signalsFromCognitiveComplexity(fc.Path, fc.Head.CognitiveComplexity)...)
		signals = append(signals, signalsFromReactOrchestration(fc.Path, fc.Head.ReactComponents)...)
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

// New constructs a Builder from options. options is copied, not aliased, so
// later mutation of the caller's Options value has no effect on the
// Builder. New cannot fail in v0.1 (no fields to validate yet); the error
// return is kept for API stability as validation is added later.
func New(options Options) (*Builder, error) {
	return &Builder{options: options}, nil
}
