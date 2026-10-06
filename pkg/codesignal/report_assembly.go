package codesignal

import (
	"github.com/lousy-agents/coach/pkg/domain"
)

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
