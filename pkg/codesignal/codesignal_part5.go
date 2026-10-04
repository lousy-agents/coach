package codesignal

import (
	"github.com/lousy-agents/coach/pkg/domain"
)

func processFileChanges(files []FileChange, seed []Diagnostic, noBaseLifecycle Lifecycle) ([]Diagnostic, []Signal) {
	diagnostics := make([]Diagnostic, 0, len(seed))
	diagnostics = append(diagnostics, seed...)
	var signals []Signal
	for _, fc := range files {
		diagnostics = append(diagnostics, validateFileChange(fc)...)

		fileDiagnostics, fileSignals := processHeadResult(fc)
		diagnostics = append(diagnostics, fileDiagnostics...)

		rangeDiagnostics, validRanges := validateChangedRanges(fc)
		diagnostics = append(diagnostics, rangeDiagnostics...)

		if !eligibleForLifecycleClassification(fc) {
			continue
		}
		fileClassifiedSignals := classifyFileSignals(baseUsableForLifecycle(fc), fileSignals, extractBaseSignals(fc), noBaseLifecycleForFile(fc, noBaseLifecycle))
		for i := range fileClassifiedSignals {
			fileClassifiedSignals[i].SourceScope = fc.SourceScope
		}
		signals = append(signals, markChanged(fileClassifiedSignals, validRanges)...)
	}
	return diagnostics, signals
}
func assembleReport(
	options Options,
	input Input,
	signals []Signal,
	diagnostics []Diagnostic,
	summary Summary,
	projectChanges []ProjectChange,
	projectFacts []ProjectFact,
	projectSummary *ProjectSummary,
	projectCoverage *domain.Coverage,
) *Report {
	scope := input.Scope
	scope.Baseline = options.Baseline
	schemaVersion := "1"
	if options.ProjectEnabled {
		schemaVersion = "2"
	}
	report := &Report{
		SchemaVersion: schemaVersion,
		Scope:         scope,
		Summary:       summary,
		Signals:       signals,
		Diagnostics:   diagnostics,
		Coverage:      input.Coverage,
	}
	if options.ProjectEnabled {
		report.ProjectChanges = projectChanges
		report.ProjectFacts = projectFacts
		report.ProjectSummary = projectSummary
		report.ProjectCoverage = projectCoverage

		prov := buildProjectProvenance(input, options)
		report.ProjectProvenance = prov
		report.ProjectScope = buildProjectScope(input)
		cnm := isCompleteNoMatch(options, prov, input, projectSummary, projectChanges)
		report.ProjectNextActions = buildNextActions(options, cnm, diagnostics)
	}
	return report
}
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
func distinctDiagnosticPaths(diagnostics []Diagnostic) map[string]struct{} {
	paths := make(map[string]struct{})
	for _, d := range diagnostics {
		if d.Path != "" {
			paths[d.Path] = struct{}{}
		}
	}
	return paths
}
func eligibleForLifecycleClassification(fc FileChange) bool {
	if fc.Head != nil {
		return fc.Head.ParseStatus == "ok"
	}
	return fc.Status == "removed"
}
