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
// not. When ProjectProvenance is absent, a project_backend_unavailable
// diagnostic is treated as incomplete so schema-1 reports (which carry no
// provenance) still satisfy the predicate.
func RequiredCoverageIncomplete(report *Report) bool {
	if report == nil {
		return false
	}
	if report.ProjectProvenance != nil {
		return !requiredProvenanceComplete(report.ProjectProvenance)
	}
	return hasProjectBackendUnavailable(report.Diagnostics)
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
