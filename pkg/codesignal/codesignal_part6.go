package codesignal

import (
	"context"

	"github.com/lousy-agents/coach/pkg/domain"
)

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
func noBaseLifecycleForFile(fc FileChange, noBaseLifecycle Lifecycle) Lifecycle {
	if fc.Status == "added" && noBaseLifecycle != "baseline" {
		return "introduced"
	}
	return noBaseLifecycle
}
func lifecycleWithoutBase(baseline bool) Lifecycle {
	if baseline {
		return Lifecycle("baseline")
	}
	return Lifecycle("unknown")
}
func extractBaseSignals(fc FileChange) []Signal {
	if !baseUsableForLifecycle(fc) || fc.Base.ParseStatus != "ok" {
		return nil
	}
	counts := findingCountsByKind(fc.Base.Findings)
	signals := signalsFromFindings(fc.Path, fc.Base.Findings, counts)
	signals = append(signals, signalsFromMetrics(fc.Path, fc.Base.Metrics)...)
	signals = append(signals, signalsFromImports(fc.Path, fc.Base.Language, fc.Base.Imports)...)
	signals = append(signals, signalsFromCognitiveComplexity(fc.Path, fc.Base.CognitiveComplexity)...)
	signals = append(signals, signalsFromReactOrchestration(fc.Path, fc.Base.ReactComponents)...)
	return signals
}
