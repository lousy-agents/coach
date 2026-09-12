// Package projectmodel defines deterministic, offline, whole-repository
// structural facts about a codebase. It may depend on pkg/semantics for
// source parsing, but it never imports pkg/codesignal or any GitHub-related
// package, so a consumer that only needs raw project facts never pulls in
// analysis policy or a GitHub client.
package projectmodel

const SchemaVersion = "1"

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

type Snapshot struct {
	Revision           string   `json:"revision"`
	TreeID             string   `json:"tree_id"`
	ConfigDigest       string   `json:"config_digest"`
	BackendDigest      string   `json:"backend_digest"`
	BuildContextDigest string   `json:"build_context_digest,omitempty"`
	SelectedRoots      []string `json:"selected_roots,omitempty"`
}

type Workspace struct {
	ID       string   `json:"id"`
	Language string   `json:"language"`
	Root     string   `json:"root"`
	Projects []string `json:"projects,omitempty"`
}

type Module struct {
	ID       string   `json:"id"`
	Path     string   `json:"path"`
	Language string   `json:"language"`
	Files    []string `json:"files,omitempty"`
}

type Package struct {
	ID       string   `json:"id"`
	Path     string   `json:"path"`
	Language string   `json:"language"`
	Files    []string `json:"files,omitempty"`
}

type File struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Language    string `json:"language"`
	BlobHash    string `json:"blob_hash,omitempty"`
	ContentHash string `json:"content_hash,omitempty"`
}

type ImportEdge struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind"`
	Site       string `json:"site,omitempty"`
	Resolution string `json:"resolution,omitempty"`
}

type CallFact struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type RootScope struct {
	Root            string   `json:"root"`
	CandidateFiles  int      `json:"candidate_files"`
	AnalyzedFiles   int      `json:"analyzed_files"`
	AnalyzedPaths   []string `json:"analyzed_paths,omitempty"`
	UnanalyzedPaths []string `json:"unanalyzed_paths,omitempty"`
}
