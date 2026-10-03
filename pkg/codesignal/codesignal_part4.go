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
func countUnanalyzedFiles(files []FileChange, diagnostics []Diagnostic) int {
	inFiles := make(map[string]bool, len(files))
	for _, fc := range files {
		if fc.Path != "" {
			inFiles[fc.Path] = true
		}
	}
	count := 0
	for path := range distinctDiagnosticPaths(diagnostics) {
		if !inFiles[path] {
			count++
		}
	}
	return count
}
func baseUsableForLifecycle(fc FileChange) bool {
	if fc.Base == nil {
		return false
	}
	return fc.Base.Path == "" || fc.Base.Path == fc.Path
}
