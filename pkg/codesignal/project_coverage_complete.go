package codesignal

// requiredSideComplete reports whether one analyzed side's required coverage
// is complete: model must be "complete", and bypass must be "complete" or
// "not_requested". Reachability is not part of required coverage.
func requiredSideComplete(model, bypass string) bool {
	return model == "complete" && (bypass == "complete" || bypass == "not_requested")
}

// isRequiredCoverageComplete reports whether required project coverage is
// complete for all analyzed sides. Required coverage tracks model and bypass
// only; reachability incompleteness alone does not block completeness.
//
// A side's coverage is required-complete when its model phase is "complete"
// and its bypass phase is "complete" or "not_requested".
func isRequiredCoverageComplete(headModel, headBypass, baseModel, baseBypass string, projectBaseAnalyzed bool) bool {
	if !requiredSideComplete(headModel, headBypass) {
		return false
	}
	if !projectBaseAnalyzed {
		return true
	}
	return requiredSideComplete(baseModel, baseBypass)
}

// RequiredCoverageIncomplete reports whether required project coverage is
// incomplete for any analyzed side of the report. Model or bypass
// incompleteness on any side counts; reachability incompleteness alone does
// not.
//
// A report with provenance (TypeScript) is judged by its model and bypass
// phases. A report without it, which is every schema-1 report and every Go
// report, is incomplete when a project_backend_unavailable diagnostic is
// present or its own completeness signals say so; see
// provenanceFreeCoverageIncomplete.
func RequiredCoverageIncomplete(report *Report) bool {
	if report == nil {
		return false
	}
	if report.ProjectProvenance != nil {
		return !requiredProvenanceComplete(report.ProjectProvenance)
	}
	return hasProjectBackendUnavailable(report.Diagnostics) || provenanceFreeCoverageIncomplete(report)
}

// provenanceFreeCoverageIncomplete reads completeness from what a report
// without provenance does carry. Its project_coverage already folds the model
// and any required-layer bypass search into one complete flag. The base side
// of a diff is not in project_coverage, so an incomplete base arrives as a
// coverage or lifecycle diagnostic, the same signals the text verdict uses to
// qualify an all-clear.
func provenanceFreeCoverageIncomplete(report *Report) bool {
	if report.ProjectCoverage != nil && !report.ProjectCoverage.Complete {
		return true
	}
	return HasProjectCoverageDiagnostic(report.Diagnostics)
}

// HasProjectCoverageDiagnostic reports whether diags carry a signal that
// project coverage was incomplete on either analyzed side. The exit-3
// predicate and the text verdict both use it, so they cannot disagree.
func HasProjectCoverageDiagnostic(diags []Diagnostic) bool {
	for _, d := range diags {
		if d.Kind == DiagKindProjectCoverageIncomplete || d.Kind == DiagKindProjectLifecycleIndeterminate {
			return true
		}
	}
	return false
}

func requiredProvenanceComplete(prov *ProjectProvenance) bool {
	var baseModel, baseBypass string
	baseAnalyzed := prov.Base != nil
	if baseAnalyzed {
		baseModel = prov.Base.Coverage.Model
		baseBypass = prov.Base.Coverage.Bypass
	}
	return isRequiredCoverageComplete(
		prov.Head.Coverage.Model, prov.Head.Coverage.Bypass,
		baseModel, baseBypass, baseAnalyzed,
	)
}

func hasProjectBackendUnavailable(diags []Diagnostic) bool {
	for _, d := range diags {
		if d.Kind == "project_backend_unavailable" {
			return true
		}
	}
	return false
}
