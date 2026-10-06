package codesignal

import (
	"github.com/lousy-agents/coach/pkg/domain"
)

func buildProjectReportSurface(input Input, noBaseLifecycle Lifecycle, includeResolved bool) (
	projectChanges []ProjectChange,
	projectFacts []ProjectFact,
	projectSummary *ProjectSummary,
	projectCoverage *domain.Coverage,
	projectSignals []Signal,
	diagnostics []Diagnostic,
) {
	lifecycleIndeterminate, diagnostics := projectLifecycleState(input)

	projectChanges, classifyDiags := classifyProjectChanges(
		input.ProjectBaseAnalyzed,
		lifecycleIndeterminate,
		input.ProjectChanges,
		input.BaseProjectChanges,
		noBaseLifecycle,
	)
	diagnostics = append(diagnostics, classifyDiags...)

	projectChanges, missingPathDiags := filterAnchorlessProjectChanges(projectChanges)
	diagnostics = append(diagnostics, missingPathDiags...)

	summaryCounts := ProjectSummary{}
	for _, change := range projectChanges {
		switch change.Lifecycle {
		case "introduced":
			summaryCounts.IntroducedChanges++
		case "existing":
			summaryCounts.ExistingChanges++
		case "resolved":
			summaryCounts.ResolvedChanges++
		case "baseline":
			summaryCounts.BaselineChanges++
		}
		// Canonicalize nested arrays before mirroring onto Signal so the
		// signals[] surface is producer-order independent, matching the
		// project_changes[] canonicalization sortProjectChanges applies below.
		projectSignals = append(projectSignals, signalFromProjectChange(withCanonicalProjectChangeArrays(change)))
	}

	if !includeResolved {
		projectChanges = withoutResolvedProjectChanges(projectChanges)
	}

	projectChanges = sortProjectChanges(projectChanges)
	summaryCounts.ActiveChanges = len(projectChanges)
	projectSummary = &summaryCounts

	projectFacts = sortProjectFacts(append([]ProjectFact(nil), input.ProjectFacts...))
	projectCoverage = cloneProjectCoverage(input.ProjectCoverage)
	return projectChanges, projectFacts, projectSummary, projectCoverage, projectSignals, diagnostics
}

func withoutResolvedProjectChanges(projectChanges []ProjectChange) []ProjectChange {
	filtered := projectChanges[:0]
	for _, change := range projectChanges {
		if change.Lifecycle == "resolved" {
			continue
		}
		filtered = append(filtered, change)
	}
	return filtered
}

func filterAnchorlessProjectChanges(changes []ProjectChange) ([]ProjectChange, []Diagnostic) {
	anchored := changes[:0]
	var diagnostics []Diagnostic
	for _, change := range changes {
		if change.PrimaryAnchor.Path == "" {
			diagnostics = append(diagnostics, Diagnostic{
				Kind: "project_observation_missing_primary_path",
				Message: "project observation semantic_key \"" + change.SemanticKey +
					"\" omitted from active project findings: primary_anchor.path is empty",
			})
			continue
		}
		anchored = append(anchored, change)
	}
	return anchored, diagnostics
}

// signalFromProjectChange projects a classified project observation onto the
// shared Signal surface so consumers that only read signals/summary still see
// active cross-module findings. machine_evidence, related_locations,
// path_steps, and coverage_refs are mirrored onto the Signal so signals-only
// consumers get text-parity evidence; project-only identity fields
// (backend_version, algorithm_version, config_digest,
// causal_evidence_digest) stay on ProjectChange.
func signalFromProjectChange(change ProjectChange) Signal {
	return Signal{
		ID:             change.ID,
		Fingerprint:    change.Fingerprint,
		RuleID:         change.RuleID,
		RuleVersion:    change.RuleVersion,
		Kind:           change.Kind,
		Category:       change.Category,
		Severity:       change.Severity,
		Confidence:     change.Confidence,
		Lifecycle:      change.Lifecycle,
		Changed:        change.Changed,
		Path:           change.PrimaryAnchor.Path,
		Subject:        change.SemanticKey,
		Location:       change.PrimaryAnchor.Location,
		Evidence:       change.Evidence,
		WhyItMatters:   change.WhyItMatters,
		Recommendation: change.Recommendation,
		SuggestedSkill: change.SuggestedSkill,
		Provenance:     change.Provenance,

		MachineEvidence:  change.MachineEvidence,
		RelatedLocations: change.RelatedLocations,
		PathSteps:        change.PathSteps,
		CoverageRefs:     change.CoverageRefs,
	}
}
