// Package domain holds project facts that both the use-case layer
// (pkg/codesignal) and the adapter that builds them (pkg/projectmodel)
// are allowed to depend on. The types live here so a use case can read
// those facts without importing the adapter.
package domain

import (
	"encoding/json"
	"sort"
)

// SchemaVersion is the project-model document version.
const SchemaVersion = "1"

// Coverage reports how completely a Model's facts were collected for a
// Snapshot, separate from and never referencing pkg/codesignal's own
// coverage/diagnostic types -- this is a project-model-native contract.
//
// Counts and Budgets are Go maps, but encoding/json sorts map keys
// alphabetically on marshal, so their JSON key order is deterministic
// regardless of insertion order; see acceptance_test.go for the test that
// pins this behavior.
type Coverage struct {
	Phase       string         `json:"phase"`
	Complete    bool           `json:"complete"`
	Counts      map[string]int `json:"counts,omitempty"`
	Budgets     map[string]int `json:"budgets,omitempty"`
	Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
}

// Diagnostic is a single project-analysis failure or limitation
// encountered while building a Model (e.g. an unresolved import).
type Diagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

// DiagBackendUnavailable must stay byte-identical to the
// "project_backend_unavailable" string embedded in
// internal/codesignalcli.ProjectBackendUnavailableError's message.
const DiagBackendUnavailable = "project_backend_unavailable"

// DiagRootScopeIncomplete marks a TypeScript root whose analyzed set does
// not cover every candidate file.
const DiagRootScopeIncomplete = "project_root_scope_incomplete"

// ReachabilityAlgorithm identifies the pinned deterministic possible-call-
// reachability traversal. It is distinct from the call-graph algorithm:
// the two evolve independently.
const ReachabilityAlgorithm = "go-source-sink-registry@1"

// TSReachabilityAlgorithm identifies the TypeScript sidecar's own
// possible-call-reachability traversal, matching the exact string
// js/semantics/src/project-sidecar/reachability-registry.ts's
// REACHABILITY_ALGORITHM constant emits on every ReachabilityFactWire.
// Go and TypeScript must report the same algorithm-version string only
// when describing the same wire-produced facts, not two independently
// invented identifiers. It is distinct from ReachabilityAlgorithm: the
// two traversal implementations evolve independently.
const TSReachabilityAlgorithm = "ts-source-sink-registry@1"

// KindPossibleCallReachability is ReachabilityFact.Kind's fixed value.
const KindPossibleCallReachability = "possible_call_reachability"

// Stable diagnostic codes for ReachabilityResult.Coverage.Diagnostics[i].Code.
const (
	DiagReachabilityBudgetExceeded   = "project_reachability_budget_exceeded"
	DiagReachabilitySourceLoadFailed = "project_reachability_source_load_failed"
)

// ReachabilityConfidence classifies how a ReachabilityFact's Path was
// derived.
type ReachabilityConfidence string

// ReachabilityConfidenceResolvedDirect is the value produced when every hop
// in Path is a direct, statically resolved edge.
const ReachabilityConfidenceResolvedDirect ReachabilityConfidence = "resolved_direct"

// ReachabilityStep is one node in a ReachabilityFact's Path, ordered from
// source to sink.
type ReachabilityStep struct {
	NodeID string `json:"node_id"`
}

// ReachabilityFact is one possible-call-reachability observation: a source
// registry entry has a statically resolved call path to a sink registry
// entry. It is facts-only. The absence of a ReachabilityFact for a given
// source/sink pair means only "no path was found within the coverage this
// run achieved"; it is never a "safe" claim.
type ReachabilityFact struct {
	ID               string                 `json:"id"`
	Kind             string                 `json:"kind"`
	Confidence       ReachabilityConfidence `json:"confidence"`
	Source           string                 `json:"source"`
	Sink             string                 `json:"sink"`
	Path             []ReachabilityStep     `json:"path"`
	AlgorithmVersion string                 `json:"algorithm_version"`
}

// ReachabilityResult is the bounded, versioned set of ReachabilityFacts
// for a snapshot, plus the coverage of that search.
type ReachabilityResult struct {
	Facts     []ReachabilityFact
	Sources   []string
	Algorithm string
	Coverage  Coverage
}

// LayerBypassAlgorithm identifies the Go layer-bypass traversal.
const LayerBypassAlgorithm = "go-layer-bypass-registry@1"

// TSLayerBypassAlgorithm identifies the TypeScript layer-bypass traversal.
const TSLayerBypassAlgorithm = "ts-layer-bypass-registry@1"

// Stable diagnostic codes for LayerBypassResult.Coverage.Diagnostics[i].Code.
const (
	DiagLayerBypassBudgetExceeded   = "project_layer_bypass_budget_exceeded"
	DiagLayerBypassSourceLoadFailed = "project_layer_bypass_source_load_failed"
	// DiagLayerBypassAmbiguousLayer marks a required layer that could not be
	// applied: either it has no prefixes, or its prefixes match no local
	// package in the snapshot. Witnesses for that run are suppressed rather
	// than reporting an ordinary reachable path as a bypass.
	DiagLayerBypassAmbiguousLayer = "project_layer_bypass_ambiguous_layer"
)

// LayerBypassConfidence classifies how a LayerBypassWitness's Path was
// derived.
type LayerBypassConfidence string

// LayerBypassConfidenceHigh is the only confidence at which a witness is
// emitted: every hop is a direct, statically resolved edge and the required
// layer was unambiguous.
const LayerBypassConfidenceHigh LayerBypassConfidence = "high"

// LayerBypassStep is one node in a LayerBypassWitness's Path, ordered from
// source to sink. Path and Line carry that node's repository-relative
// declaration position so a consumer can anchor on source instead of the
// NodeID alone. Both stay zero when the node has no resolvable local
// position, which is expected for a stdlib sink.
type LayerBypassStep struct {
	NodeID string `json:"node_id"`
	Path   string `json:"path,omitempty"`
	Line   int    `json:"line,omitempty"`
}

// BypassLayer names a required intermediate layer and its repository-relative
// directory prefixes. "." matches every directory; otherwise a prefix matches
// a directory that equals it or has it as a "/"-separated ancestor.
type BypassLayer struct {
	Name     string
	Prefixes []string
}

// LayerBypassWitness is one layer-bypass observation: a statically resolved
// call path from Source to Sink that survives removing every node classified
// under RequiredLayer. Absence of a witness means only that no bypass path
// was found within this run's coverage.
type LayerBypassWitness struct {
	ID               string                `json:"id"`
	Source           string                `json:"source"`
	Sink             string                `json:"sink"`
	RequiredLayer    string                `json:"required_layer"`
	Path             []LayerBypassStep     `json:"path"`
	Confidence       LayerBypassConfidence `json:"confidence"`
	AlgorithmVersion string                `json:"algorithm_version"`
}

// LayerBypassResult is the bounded set of LayerBypassWitnesses for a
// snapshot, plus the coverage of that search.
type LayerBypassResult struct {
	Witnesses []LayerBypassWitness
	Sources   []string
	Algorithm string
	Coverage  Coverage
}

// InclusionRuleTSConfigIncludesNoTestClassification is the inclusion rule
// recorded on a TypeScript ProjectScope.
const InclusionRuleTSConfigIncludesNoTestClassification = "tsconfig_includes_no_test_classification"

// ProjectScopeRoot is one policy root's candidate and analyzed file counts.
type ProjectScopeRoot struct {
	Root           string `json:"root"`
	CandidateFiles int    `json:"candidate_files"`
	AnalyzedFiles  int    `json:"analyzed_files"`
}

// ProjectScope is the per-revision scope classification: which policy roots
// were analyzed and which configured layers matched those files.
type ProjectScope struct {
	InclusionRule   string             `json:"inclusion_rule"`
	PatternSet      string             `json:"pattern_set"`
	Roots           []ProjectScopeRoot `json:"roots"`
	MatchedLayers   []string           `json:"matched_layers"`
	UnmatchedLayers []string           `json:"unmatched_layers"`
}

// Model is the whole-repository structural fact document.
type Model struct {
	SchemaVersion string       `json:"schema_version"`
	Repository    string       `json:"repository,omitempty"`
	Snapshot      Snapshot     `json:"snapshot"`
	Workspaces    []Workspace  `json:"workspaces,omitempty"`
	Modules       []Module     `json:"modules,omitempty"`
	Packages      []Package    `json:"packages,omitempty"`
	Files         []File       `json:"files,omitempty"`
	ImportEdges   []ImportEdge `json:"import_edges,omitempty"`
	CallFacts     []CallFact   `json:"call_facts,omitempty"`

	ReachabilityFacts []ReachabilityFact `json:"reachability_facts,omitempty"`
	RootScopes        []RootScope        `json:"root_scopes,omitempty"`

	Coverage Coverage `json:"coverage"`
}

// Snapshot identifies the revision and analysis identity a Model was built from.
type Snapshot struct {
	Revision           string   `json:"revision"`
	TreeID             string   `json:"tree_id"`
	ConfigDigest       string   `json:"config_digest"`
	BackendDigest      string   `json:"backend_digest"`
	BuildContextDigest string   `json:"build_context_digest,omitempty"`
	SelectedRoots      []string `json:"selected_roots,omitempty"`
}

// Workspace is one language workspace discovered in a snapshot.
type Workspace struct {
	ID       string   `json:"id"`
	Language string   `json:"language"`
	Root     string   `json:"root"`
	Projects []string `json:"projects,omitempty"`
}

// Module is one module discovered in a snapshot.
type Module struct {
	ID       string   `json:"id"`
	Path     string   `json:"path"`
	Language string   `json:"language"`
	Files    []string `json:"files,omitempty"`
}

// Package is one package discovered in a snapshot.
type Package struct {
	ID       string   `json:"id"`
	Path     string   `json:"path"`
	Language string   `json:"language"`
	Files    []string `json:"files,omitempty"`
}

// File is one source file recorded in a Model.
type File struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Language    string `json:"language"`
	BlobHash    string `json:"blob_hash,omitempty"`
	ContentHash string `json:"content_hash,omitempty"`
}

// ImportEdge is one resolved import from one package or file to another.
type ImportEdge struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind"`
	Site       string `json:"site,omitempty"`
	Resolution string `json:"resolution,omitempty"`
}

// CallFact is one statically resolved call edge.
type CallFact struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// RootScope is the candidate and analyzed file set for one policy root.
type RootScope struct {
	Root            string   `json:"root"`
	CandidateFiles  int      `json:"candidate_files"`
	AnalyzedFiles   int      `json:"analyzed_files"`
	AnalyzedPaths   []string `json:"analyzed_paths,omitempty"`
	UnanalyzedPaths []string `json:"unanalyzed_paths,omitempty"`
}

// ModelWire mirrors Model's JSON shape but carries CallFacts and
// ReachabilityFacts as raw JSON so MarshalJSON/UnmarshalJSON can distinguish
// "key absent" (nil slice) from "key present as an empty array".
// encoding/json's omitempty treats a nil slice and a zero-length slice
// identically. Every field of Model must be mirrored here; a field added to
// Model but not to ModelWire is silently dropped from JSON output.
type ModelWire struct {
	SchemaVersion     string          `json:"schema_version"`
	Repository        string          `json:"repository,omitempty"`
	Snapshot          Snapshot        `json:"snapshot"`
	Workspaces        []Workspace     `json:"workspaces,omitempty"`
	Modules           []Module        `json:"modules,omitempty"`
	Packages          []Package       `json:"packages,omitempty"`
	Files             []File          `json:"files,omitempty"`
	ImportEdges       []ImportEdge    `json:"import_edges,omitempty"`
	CallFacts         json.RawMessage `json:"call_facts,omitempty"`
	ReachabilityFacts json.RawMessage `json:"reachability_facts,omitempty"`
	RootScopes        []RootScope     `json:"root_scopes,omitempty"`
	Coverage          Coverage        `json:"coverage"`
}

func (m *Model) UnmarshalJSON(data []byte) error {
	var wire ModelWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*m = Model{
		SchemaVersion: wire.SchemaVersion,
		Repository:    wire.Repository,
		Snapshot:      wire.Snapshot,
		Workspaces:    wire.Workspaces,
		Modules:       wire.Modules,
		Packages:      wire.Packages,
		Files:         wire.Files,
		ImportEdges:   wire.ImportEdges,
		RootScopes:    wire.RootScopes,
		Coverage:      wire.Coverage,
	}
	if len(wire.CallFacts) > 0 {
		var facts []CallFact
		if err := json.Unmarshal(wire.CallFacts, &facts); err != nil {
			return err
		}
		m.CallFacts = facts
	}
	if len(wire.ReachabilityFacts) > 0 {
		var facts []ReachabilityFact
		if err := json.Unmarshal(wire.ReachabilityFacts, &facts); err != nil {
			return err
		}
		m.ReachabilityFacts = facts
	}
	return nil
}

func canonicalFiles(in []File) []File {
	if len(in) == 0 {
		return in
	}
	out := append([]File(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		if a.BlobHash != b.BlobHash {
			return a.BlobHash < b.BlobHash
		}
		if a.ContentHash != b.ContentHash {
			return a.ContentHash < b.ContentHash
		}
		return a.Language < b.Language
	})
	return out
}

// canonicalRootScopes assumes Root is unique per Model. A duplicate Root is
// unsupported input with unspecified relative order.
