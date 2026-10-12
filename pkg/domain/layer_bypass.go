package domain

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
