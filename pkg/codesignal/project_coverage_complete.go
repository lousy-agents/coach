package codesignal

import "slices"

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
	return requiredSideComplete(headModel, headBypass) &&
		(!projectBaseAnalyzed || requiredSideComplete(baseModel, baseBypass))
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
	if hasWorktreeScanIncomplete(report.Scope, report.Diagnostics) {
		return true
	}
	if report.ProjectProvenance != nil {
		return !requiredProvenanceComplete(report.ProjectProvenance)
	}
	return hasProjectBackendUnavailable(report.Diagnostics) || provenanceFreeCoverageIncomplete(report)
}

// provenanceFreeCoverageIncomplete reads completeness from what a report
// without provenance does carry. Its project_coverage already folds the model
// and any required-layer bypass search into one complete flag. The base side
// of a diff is not in project_coverage, so an incomplete base side arrives as
// a project_lifecycle_indeterminate diagnostic; project_coverage_incomplete is
// raised for the head side only. Both are signals the text verdict uses to
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
	return slices.ContainsFunc(diags, func(d Diagnostic) bool {
		return d.Kind == DiagKindProjectCoverageIncomplete || d.Kind == DiagKindProjectLifecycleIndeterminate
	})
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
	return slices.ContainsFunc(diags, func(d Diagnostic) bool {
		return d.Kind == "project_backend_unavailable"
	})
}

// hasWorktreeScanIncomplete reports whether a --worktree scan recorded that
// part of the working tree could not be read, which makes the scan incomplete
// in every language. Four kinds say so: a skipped symbolic link, a changed
// path that nothing analyzed or accounted for, Git state the scan cannot see
// through (sparse checkout, skip-worktree, assume-unchanged, a dirty
// submodule or an untracked nested repository), and a failed status read.
//
// Other diagnostics that qualify a --worktree verdict, such as
// unsupported_language and continuity_not_determined, describe what the scan
// understood rather than what it could not read. They keep their place in the
// verdict text and leave the exit status alone, as they do in a committed scan;
// counting them would fail every run over an edited README or a renamed file.
// Outside --worktree the same kinds describe a committed scan of a dirty tree
// and leave the exit status alone as well.
func hasWorktreeScanIncomplete(scope Scope, diags []Diagnostic) bool {
	if scope.Worktree == nil {
		return false
	}
	for _, d := range diags {
		switch d.Kind {
		case DiagKindWorktreeScanIncomplete, DiagKindWorktreeSymlinkSkipped,
			DiagKindWorktreeChangesNotAnalyzed, DiagKindWorktreeStatusCheckFailed:
			return true
		}
	}
	return false
}
