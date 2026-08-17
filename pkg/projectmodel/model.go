// Package projectmodel defines deterministic, offline, whole-repository
// structural facts about a codebase: workspace/module/package/file
// inventory and import edges. It holds no policy and no derived signals --
// it may depend on pkg/semantics for source parsing, but it never imports
// pkg/codesignal or any GitHub-related package, so a consumer that only
// needs raw project facts never pulls in analysis policy or a GitHub
// client.
package projectmodel

// SchemaVersion is the frozen schema version for Model's JSON encoding.
const SchemaVersion = "1"

// Model is the top-level, JSON-serializable set of facts collected for a
// single Snapshot. json.Marshal on a Model is its canonical encoding: see
// serialization.go, which sorts workspaces, modules, packages, files,
// import edges, call facts, reachability facts, and coverage diagnostics so
// semantically identical models marshal byte-identically regardless of
// producer order.
type Model struct {
	SchemaVersion string       `json:"schema_version"`
	Repository    string       `json:"repository,omitempty"`
	Snapshot      Snapshot     `json:"snapshot"`
	Workspaces    []Workspace  `json:"workspaces,omitempty"`
	Modules       []Module     `json:"modules,omitempty"`
	Packages      []Package    `json:"packages,omitempty"`
	Files         []File       `json:"files,omitempty"`
	ImportEdges   []ImportEdge `json:"import_edges,omitempty"`

	// CallFacts distinguishes "collection was not selected" (nil, omitted
	// key) from "collection was selected but found nothing" (non-nil empty
	// slice, serializes as "call_facts":[]). The `omitempty` tag alone
	// cannot express this -- see Model's MarshalJSON/UnmarshalJSON in
	// serialization.go, which encode/decode this field explicitly.
	CallFacts []CallFact `json:"call_facts,omitempty"`

	// ReachabilityFacts mirrors CallFacts' exact nil-vs-empty-slice JSON
	// contract: nil/omitted means reachability collection was not
	// selected, a non-nil (possibly empty) slice means it was selected.
	// BuildTypeScriptModelViaSidecar is currently the only producer, since
	// the TS sidecar computes call-graph and reachability facts in the
	// same round trip as its import facts; a Go-side equivalent may
	// populate this field the same way in the future without changing its
	// contract.
	ReachabilityFacts []ReachabilityFact `json:"reachability_facts,omitempty"`

	Coverage Coverage `json:"coverage"`
}

// Snapshot identifies the immutable source and evaluator inputs used to
// build a Model. All values are revision/content identities rather than
// absolute filesystem paths, so the same repository produces the same facts
// from any caller working directory. SelectedRoots and BuildContextDigest
// freeze the optional context #208 requires for reproducible downstream
// provenance; empty values omit from JSON.
type Snapshot struct {
	Revision           string   `json:"revision"`
	TreeID             string   `json:"tree_id"`
	ConfigDigest       string   `json:"config_digest"`
	BackendDigest      string   `json:"backend_digest"`
	BuildContextDigest string   `json:"build_context_digest,omitempty"`
	SelectedRoots      []string `json:"selected_roots,omitempty"`
}

// Workspace is a discovered language-specific project root within a
// Snapshot. Root is repository-relative and Projects contains stable IDs of
// the projects owned by the workspace.
type Workspace struct {
	ID       string   `json:"id"`
	Language string   `json:"language"`
	Root     string   `json:"root"`
	Projects []string `json:"projects,omitempty"`
}

// Module is a language-level module boundary discovered within a Snapshot.
type Module struct {
	ID       string   `json:"id"`
	Path     string   `json:"path"`
	Language string   `json:"language"`
	Files    []string `json:"files,omitempty"`
}

// Package is a language-level package/namespace boundary discovered within a
// Snapshot.
type Package struct {
	ID       string   `json:"id"`
	Path     string   `json:"path"`
	Language string   `json:"language"`
	Files    []string `json:"files,omitempty"`
}

// File is a single source file inventoried within a Snapshot. BlobHash and
// ContentHash are optional content identities (Git blob OID / content digest)
// when the backend can supply them; paths stay repository-relative.
type File struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Language    string `json:"language"`
	BlobHash    string `json:"blob_hash,omitempty"`
	ContentHash string `json:"content_hash,omitempty"`
}

// ImportEdge is a directed import relationship between two facts (files,
// packages, or modules, depending on Kind).
type ImportEdge struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind"`
	Site       string `json:"site,omitempty"`
	Resolution string `json:"resolution,omitempty"`
}

// CallFact is a directed call relationship between two symbols.
type CallFact struct {
	From string `json:"from"`
	To   string `json:"to"`
}
