package codesignal

import (
	"sort"

	"github.com/lousy-agents/coach/pkg/domain"
)

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

// Canonicalize nested arrays before mirroring onto Signal so the
// signals[] surface is producer-order independent, matching the
// project_changes[] canonicalization sortProjectChanges applies below.

// Complete coverage is required before any normal lifecycle claim.

// Non-empty base observations without ProjectBaseAnalyzed are inconsistent.
// A non-nil empty slice is not: callers commonly initialize with make/append.

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

func cloneProjectCoverage(in *domain.Coverage) *domain.Coverage {
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
		out.Diagnostics = append([]domain.Diagnostic(nil), in.Diagnostics...)
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

func completeProjectCoverage(coverage *domain.Coverage) bool {
	return coverage != nil && coverage.Complete
}

// Only blame the base side when a base model was analyzed or base
// observations were actually supplied. Baseline runs and head-only
// diffs never expect base coverage.

func countFilesWithDiagnostics(_ []FileChange, diagnostics []Diagnostic) int {
	return len(distinctDiagnosticPaths(diagnostics))
}
