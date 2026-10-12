package domain

import (
	"encoding/json"
)

// SchemaVersion is the project-model document version.
const SchemaVersion = "1"

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

func (m Model) MarshalJSON() ([]byte, error) {
	snapshot := m.Snapshot
	snapshot.SelectedRoots = sortedStrings(snapshot.SelectedRoots)
	wire := ModelWire{
		SchemaVersion: m.SchemaVersion,
		Repository:    m.Repository,
		Snapshot:      snapshot,
		Workspaces:    canonicalWorkspaces(m.Workspaces),
		Modules:       canonicalModules(m.Modules),
		Packages:      canonicalPackages(m.Packages),
		Files:         canonicalFiles(m.Files),
		ImportEdges:   canonicalImportEdges(m.ImportEdges),
		RootScopes:    canonicalRootScopes(m.RootScopes),
		Coverage:      CanonicalCoverage(m.Coverage),
	}
	if m.CallFacts != nil {
		raw, err := json.Marshal(CanonicalCallFacts(m.CallFacts))
		if err != nil {
			return nil, err
		}
		wire.CallFacts = raw
	}
	if m.ReachabilityFacts != nil {
		raw, err := json.Marshal(canonicalReachabilityFacts(m.ReachabilityFacts))
		if err != nil {
			return nil, err
		}
		wire.ReachabilityFacts = raw
	}
	return json.Marshal(wire)
}
