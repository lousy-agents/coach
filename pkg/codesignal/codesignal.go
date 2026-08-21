package codesignal

import (
	"context"
	"sort"
	"strings"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// Options configures a Builder.
type Options struct {
	IncludeResolved bool `json:"include_resolved"`
	Baseline        bool `json:"baseline"`

	// ProjectEnabled switches Build onto the schema-2 project-analysis
	// report path: SchemaVersion becomes "2" and the Report's project_*
	// fields become eligible to serialize. See Report's field block for
	// the byte-identity guarantee this default-false zero value preserves.
	ProjectEnabled bool `json:"project_enabled"`
}

// Builder produces Reports from Input. It holds no mutable state after
// construction (options is copied in New and never written to again), so a
// *Builder is safe for concurrent Build calls without additional
// synchronization.
type Builder struct {
	options Options
}

// New constructs a Builder from options. options is copied, not aliased, so
// later mutation of the caller's Options value has no effect on the
// Builder. New cannot fail in v0.1 (no fields to validate yet); the error
// return is kept for API stability as validation is added later.
func New(options Options) (*Builder, error) {
	return &Builder{options: options}, nil
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
	var projectCoverage *projectmodel.Coverage
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

func lifecycleWithoutBase(baseline bool) Lifecycle {
	if baseline {
		return Lifecycle("baseline")
	}
	return Lifecycle("unknown")
}

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
		fileClassifiedSignals := classifyFileSignals(baseUsableForLifecycle(fc), fileSignals, extractBaseSignals(fc), noBaseLifecycle)
		for i := range fileClassifiedSignals {
			fileClassifiedSignals[i].SourceScope = fc.SourceScope
		}
		markChanged(fileClassifiedSignals, validRanges)
		signals = append(signals, fileClassifiedSignals...)
	}
	return diagnostics, signals
}

func finalizeSignals(signals []Signal, filesAnalyzed int, files []FileChange, diagnostics []Diagnostic, includeResolved bool) ([]Signal, Summary) {
	summary := Summary{
		FilesAnalyzed:        filesAnalyzed,
		FilesWithDiagnostics: countFilesWithDiagnostics(files, diagnostics),
	}
	for _, sig := range signals {
		switch sig.Lifecycle {
		case "introduced":
			summary.IntroducedSignals++
		case "existing":
			summary.ExistingSignals++
		case "resolved":
			summary.ResolvedSignals++
		case "baseline":
			summary.BaselineSignals++
		}
	}
	if !includeResolved {
		filtered := make([]Signal, 0, len(signals))
		for _, sig := range signals {
			if sig.Lifecycle == "resolved" {
				continue
			}
			filtered = append(filtered, sig)
		}
		signals = filtered
	}
	sortSignals(signals)
	summary.ActiveSignals = len(signals)
	return signals, summary
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
	projectCoverage *projectmodel.Coverage,
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
	}
	return report
}

// buildProjectReportSurface classifies project observations, drops anchorless
// and (optionally) resolved entries, projects anchored findings onto the shared
// signals surface, and returns schema-2 project report fields plus diagnostics.
func buildProjectReportSurface(input Input, noBaseLifecycle Lifecycle, includeResolved bool) (
	projectChanges []ProjectChange,
	projectFacts []ProjectFact,
	projectSummary *ProjectSummary,
	projectCoverage *projectmodel.Coverage,
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
		filtered := projectChanges[:0]
		for _, change := range projectChanges {
			if change.Lifecycle == "resolved" {
				continue
			}
			filtered = append(filtered, change)
		}
		projectChanges = filtered
	}

	projectChanges = sortProjectChanges(projectChanges)
	summaryCounts.ActiveChanges = len(projectChanges)
	projectSummary = &summaryCounts

	projectFacts = sortProjectFacts(append([]ProjectFact(nil), input.ProjectFacts...))
	projectCoverage = cloneProjectCoverage(input.ProjectCoverage)
	return projectChanges, projectFacts, projectSummary, projectCoverage, projectSignals, diagnostics
}

func projectLifecycleState(input Input) (indeterminate bool, diagnostics []Diagnostic) {
	// Complete coverage is required before any normal lifecycle claim.
	if !completeProjectCoverage(input.ProjectCoverage) {
		indeterminate = true
	}
	if input.ProjectBaseAnalyzed && !completeProjectCoverage(input.BaseProjectCoverage) {
		indeterminate = true
	}
	// Non-empty base observations without ProjectBaseAnalyzed are inconsistent.
	// A non-nil empty slice is not: callers commonly initialize with make/append.
	if !input.ProjectBaseAnalyzed && len(input.BaseProjectChanges) > 0 {
		indeterminate = true
	}
	if input.ProjectCoverage != nil && !input.ProjectCoverage.Complete {
		diagnostics = append(diagnostics, Diagnostic{
			Kind:    DiagKindProjectCoverageIncomplete,
			Message: "project analysis coverage is incomplete; project observations may be partial",
		})
	}
	if indeterminate {
		diagnostics = append(diagnostics, Diagnostic{
			Kind:    DiagKindProjectLifecycleIndeterminate,
			Message: projectLifecycleDiagnosticMessage(input),
		})
	}
	return indeterminate, diagnostics
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

func cloneProjectCoverage(in *projectmodel.Coverage) *projectmodel.Coverage {
	if in == nil {
		return nil
	}
	out := *in
	if len(in.Counts) > 0 {
		out.Counts = make(map[string]int, len(in.Counts))
		for k, v := range in.Counts {
			out.Counts[k] = v
		}
	}
	if len(in.Budgets) > 0 {
		out.Budgets = make(map[string]int, len(in.Budgets))
		for k, v := range in.Budgets {
			out.Budgets[k] = v
		}
	}
	if len(in.Diagnostics) > 0 {
		out.Diagnostics = append([]projectmodel.Diagnostic(nil), in.Diagnostics...)
		sort.SliceStable(out.Diagnostics, func(i, j int) bool {
			a, b := out.Diagnostics[i], out.Diagnostics[j]
			if a.Code != b.Code {
				return a.Code < b.Code
			}
			if a.Path != b.Path {
				return a.Path < b.Path
			}
			return a.Message < b.Message
		})
	}
	return &out
}

func completeProjectCoverage(coverage *projectmodel.Coverage) bool {
	return coverage != nil && coverage.Complete
}

func projectLifecycleDiagnosticMessage(input Input) string {
	reasons := make([]string, 0, 2)
	if input.ProjectCoverage == nil {
		reasons = append(reasons, "head coverage unavailable")
	} else if !input.ProjectCoverage.Complete {
		reasons = append(reasons, "head coverage incomplete")
	}
	// Only blame the base side when a base model was analyzed or base
	// observations were actually supplied. Baseline runs and head-only
	// diffs never expect base coverage.
	baseSideExpected := input.ProjectBaseAnalyzed || len(input.BaseProjectChanges) > 0
	if baseSideExpected {
		if !input.ProjectBaseAnalyzed || input.BaseProjectCoverage == nil {
			reasons = append(reasons, "base coverage unavailable")
		} else if !input.BaseProjectCoverage.Complete {
			reasons = append(reasons, "base coverage incomplete")
		}
	}
	if len(reasons) == 0 {
		return "project lifecycle is indeterminate"
	}
	return "project lifecycle is indeterminate: " + strings.Join(reasons, "; ")
}

func processHeadResult(fc FileChange) ([]Diagnostic, []Signal) {
	if fc.Head == nil {
		if fc.Status == "modified" || fc.Status == "added" {
			return []Diagnostic{{
				Path:    fc.Path,
				Kind:    "missing_head_result",
				Message: "file change status \"" + string(fc.Status) + "\" has no head analysis result",
			}}, nil
		}
		return nil, nil
	}

	switch fc.Head.ParseStatus {
	case "ok":
		counts := findingCountsByKind(fc.Head.Findings)
		signals := signalsFromFindings(fc.Path, fc.Head.Findings, counts)
		signals = append(signals, signalsFromMetrics(fc.Path, fc.Head.Metrics)...)
		signals = append(signals, signalsFromImports(fc.Path, fc.Head.Language, fc.Head.Imports)...)
		signals = append(signals, signalsFromCognitiveComplexity(fc.Path, fc.Head.CognitiveComplexity)...)
		signals = append(signals, signalsFromReactOrchestration(fc.Path, fc.Head.ReactComponents)...)
		return nil, signals
	case "syntax_errors":
		diagnostics := make([]Diagnostic, 0, len(fc.Head.SyntaxErrors))
		for _, issue := range fc.Head.SyntaxErrors {
			location := issue.Location
			diagnostics = append(diagnostics, Diagnostic{
				Path:     fc.Path,
				Kind:     "syntax_errors",
				Location: &location,
				Message:  "head analysis found a syntax issue of kind \"" + issue.Kind + "\"",
			})
		}
		return diagnostics, nil
	default:
		return []Diagnostic{{
			Path:    fc.Path,
			Kind:    "unsupported_parse_status",
			Message: "head analysis result has unsupported parse status \"" + string(fc.Head.ParseStatus) + "\"",
		}}, nil
	}
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

func baseUsableForLifecycle(fc FileChange) bool {
	if fc.Base == nil {
		return false
	}
	return fc.Base.Path == "" || fc.Base.Path == fc.Path
}

func eligibleForLifecycleClassification(fc FileChange) bool {
	if fc.Head != nil {
		return fc.Head.ParseStatus == "ok"
	}
	return fc.Status == "removed"
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

func countFilesWithDiagnostics(files []FileChange, diagnostics []Diagnostic) int {
	withDiagnostics := make(map[string]bool, len(diagnostics))
	for _, d := range diagnostics {
		withDiagnostics[d.Path] = true
	}

	count := 0
	seen := make(map[string]bool, len(files))
	for _, fc := range files {
		if seen[fc.Path] {
			continue
		}
		seen[fc.Path] = true
		if withDiagnostics[fc.Path] {
			count++
		}
	}

	return count
}
