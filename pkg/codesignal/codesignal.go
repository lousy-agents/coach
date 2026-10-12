package codesignal

import (
	"context"

	"github.com/lousy-agents/coach/pkg/domain"
)

// Builder produces Reports from Input. It holds no mutable state after
// construction (options is copied in New and never written to again), so a
// *Builder is safe for concurrent Build calls without additional
// synchronization.
type Builder struct {
	options Options
}

func (b *Builder) Build(ctx context.Context, input Input) (*Report, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	noBaseLifecycle := lifecycleWithoutBase(b.options.Baseline)
	diagnostics, signals := processFileChanges(input.Files, input.Diagnostics, noBaseLifecycle)
	// File-level counters see only file-pipeline diagnostics: a project
	// surface Path (a degraded change's anchor, a model-coverage candidate or
	// root directory) is not a changed file that was or was not analyzed.
	fileDiagnostics := append([]Diagnostic(nil), diagnostics...)

	var projectChanges []ProjectChange
	var projectFacts []ProjectFact
	var projectSummary *ProjectSummary
	var projectCoverage *domain.Coverage
	if b.options.ProjectEnabled {
		var projectSignals []Signal
		var projectDiags []Diagnostic
		projectChanges, projectFacts, projectSummary, projectCoverage, projectSignals, projectDiags =
			buildProjectReportSurface(input, noBaseLifecycle, b.options.IncludeResolved)
		signals = append(signals, projectSignals...)
		diagnostics = append(diagnostics, projectDiags...)
	}

	sortDiagnostics(diagnostics)
	signals, summary := finalizeSignals(signals, len(input.Files), input.Files, fileDiagnostics, b.options.IncludeResolved)
	return assembleReport(b.options, input, signals, diagnostics, summary, projectChanges, projectFacts, projectSummary, projectCoverage), nil
}
