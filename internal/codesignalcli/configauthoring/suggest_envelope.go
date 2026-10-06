package configauthoring

import (
	"encoding/json"
	"sort"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

type suggestionCandidate struct {
	SchemaVersion string   `json:"schema_version"`
	Roots         []string `json:"roots"`
}

type suggestionEnvelope struct {
	DiagnosticVersion string                    `json:"diagnostic_version"`
	Kind              string                    `json:"kind"`
	Revision          string                    `json:"revision"`
	HeuristicVersion  string                    `json:"heuristic_version"`
	RootsConsidered   []string                  `json:"roots_considered"`
	Coverage          suggestCoverageWire       `json:"coverage"`
	Diagnostics       []projectmodel.Diagnostic `json:"diagnostics"`
}

// suggestCoverageWire mirrors projectmodel.Coverage for this envelope's
// stderr wire shape, with two deliberate deviations from projectmodel.
// Coverage's own JSON tags: Phase is always the mandated
// "project_config_suggestion" (issue #220), never DiscoverGoRoots' own
// "go_root_discovery" phase name, which only describes pkg/projectmodel's
// internal call; and Diagnostics has no "omitempty", so an empty slice
// marshals as the literal "[]" issue #220 (and #210, whose coverage shape
// this reuses) require, rather than being omitted entirely.
type suggestCoverageWire struct {
	Phase       string                    `json:"phase"`
	Complete    bool                      `json:"complete"`
	Counts      map[string]int            `json:"counts,omitempty"`
	Budgets     map[string]int            `json:"budgets,omitempty"`
	Diagnostics []projectmodel.Diagnostic `json:"diagnostics"`
}

func suggestCoverageWireFrom(in projectmodel.Coverage) suggestCoverageWire {
	diagnostics := in.Diagnostics
	if diagnostics == nil {
		diagnostics = []projectmodel.Diagnostic{}
	}
	return suggestCoverageWire{
		Phase:       "project_config_suggestion",
		Complete:    in.Complete,
		Counts:      in.Counts,
		Budgets:     in.Budgets,
		Diagnostics: diagnostics,
	}
}

// buildSuggestEnvelope renders the single stderr provenance/diagnostic
// document every --suggest-project-config invocation writes (except
// --help): one UTF-8 newline-delimited JSON (NDJSON) object -- compact,
// single-line, no indentation -- followed by exactly one trailing newline,
// with fixed key order. This differs deliberately from the stdout
// candidate, which is 2-space-indented multi-line JSON meant for a human to
// read and commit.
func buildSuggestEnvelope(revision string, roots []string, coverage projectmodel.Coverage, primary projectmodel.Diagnostic) []byte {
	rootsConsidered := make([]string, len(roots))
	copy(rootsConsidered, roots)
	sort.Strings(rootsConsidered)

	envelope := suggestionEnvelope{
		DiagnosticVersion: "1",
		Kind:              "project_config_suggestion",
		Revision:          revision,
		HeuristicVersion:  "go-project-config-roots@1",
		RootsConsidered:   rootsConsidered,
		Coverage:          suggestCoverageWireFrom(coverage),
		Diagnostics:       []projectmodel.Diagnostic{primary},
	}
	// envelope's fields are plain strings/bools/maps[string]int/slices of
	// small structs, so Marshal cannot fail for this shape.
	data, _ := json.Marshal(envelope)
	return append(data, '\n')
}

// zeroSuggestCoverage synthesizes the Coverage shape reported before any
// DiscoverGoRoots result exists (invalid-arguments, output-path validation,
// and snapshot-open failures), matching the counts/budgets vocabulary
// DiscoverGoRoots itself reports.
func zeroSuggestCoverage() projectmodel.Coverage {
	return projectmodel.Coverage{
		// Phase is overwritten by buildSuggestEnvelope's suggestCoverageWireFrom
		// regardless of what is set here; left as "project_config_suggestion"
		// for readability at call sites that inspect this value directly.
		Phase:    "project_config_suggestion",
		Complete: false,
		Counts: map[string]int{
			"files_seen":      0,
			"files_skipped":   0,
			"modules_seen":    0,
			"modules_skipped": 0,
			"roots_emitted":   0,
		},
		Budgets: projectmodel.EffectiveGoBudgets(suggestGoBudgets),
	}
}

// normalizeSuggestionRoots defensively re-sorts and deduplicates
// DiscoverGoRoots' already-sorted, deduplicated Roots so the candidate
// contract holds even if that upstream invariant is ever relaxed.
func normalizeSuggestionRoots(roots []string) []string {
	out := make([]string, 0, len(roots))
	out = append(out, roots...)
	sort.Strings(out)
	deduped := out[:0]
	var previous string
	for i, root := range out {
		if i == 0 || root != previous {
			deduped = append(deduped, root)
			previous = root
		}
	}
	return deduped
}

// serializeSuggestionCandidate renders roots as the strict schema-1
// project-config candidate: 2-space indent, one trailing newline, fixed
// key order, sorted and deduplicated roots.
func serializeSuggestionCandidate(roots []string) ([]byte, error) {
	candidate := suggestionCandidate{
		SchemaVersion: "1",
		Roots:         normalizeSuggestionRoots(roots),
	}
	data, err := json.MarshalIndent(candidate, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
