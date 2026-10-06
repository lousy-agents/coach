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
	signals, summary := finalizeSignals(signals, len(input.Files), input.Files, diagnostics, b.options.IncludeResolved)
	return assembleReport(b.options, input, signals, diagnostics, summary, projectChanges, projectFacts, projectSummary, projectCoverage), nil
}
