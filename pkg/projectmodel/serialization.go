package projectmodel

import (
	"encoding/json"
	"sort"
)

// modelWire mirrors Model's JSON shape but carries CallFacts as raw JSON so
// MarshalJSON/UnmarshalJSON can distinguish "key absent" (CallFacts nil)
// from "key present as an empty array" (CallFacts non-nil, empty) -- a
// plain []CallFact field with `omitempty` cannot make that distinction,
// since encoding/json's omitempty treats a nil slice and a zero-length
// slice identically. Every field of Model must be mirrored here; a field
// added to Model but not to modelWire is silently dropped from JSON output.
type modelWire struct {
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

func (m Model) MarshalJSON() ([]byte, error) {
	snapshot := m.Snapshot
	snapshot.SelectedRoots = sortedStrings(snapshot.SelectedRoots)
	wire := modelWire{
		SchemaVersion: m.SchemaVersion,
		Repository:    m.Repository,
		Snapshot:      snapshot,
		Workspaces:    canonicalWorkspaces(m.Workspaces),
		Modules:       canonicalModules(m.Modules),
		Packages:      canonicalPackages(m.Packages),
		Files:         canonicalFiles(m.Files),
		ImportEdges:   canonicalImportEdges(m.ImportEdges),
		RootScopes:    canonicalRootScopes(m.RootScopes),
		Coverage:      canonicalCoverage(m.Coverage),
	}
	if m.CallFacts != nil {
		raw, err := json.Marshal(canonicalCallFacts(m.CallFacts))
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

func (m *Model) UnmarshalJSON(data []byte) error {
	var wire modelWire
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

func canonicalWorkspaces(in []Workspace) []Workspace {
	if len(in) == 0 {
		return in
	}
	out := append([]Workspace(nil), in...)
	for i := range out {
		out[i].Projects = sortedStrings(out[i].Projects)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		if out[i].Root != out[j].Root {
			return out[i].Root < out[j].Root
		}
		return out[i].Language < out[j].Language
	})
	return out
}

func canonicalModules(in []Module) []Module {
	if len(in) == 0 {
		return in
	}
	out := append([]Module(nil), in...)
	for i := range out {
		out[i].Files = sortedStrings(out[i].Files)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func canonicalPackages(in []Package) []Package {
	if len(in) == 0 {
		return in
	}
	out := append([]Package(nil), in...)
	for i := range out {
		out[i].Files = sortedStrings(out[i].Files)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Path < out[j].Path
	})
	return out
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

func canonicalImportEdges(in []ImportEdge) []ImportEdge {
	if len(in) == 0 {
		return in
	}
	out := append([]ImportEdge(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.To != b.To {
			return a.To < b.To
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Site != b.Site {
			return a.Site < b.Site
		}
		return a.Resolution < b.Resolution
	})
	return out
}

// canonicalRootScopes assumes Root is unique per Model, mirroring
// project_scope.go's scopeByRoot lookup (last write wins on a duplicate);
// a duplicate Root is unsupported input with unspecified relative order.
func canonicalRootScopes(in []RootScope) []RootScope {
	if len(in) == 0 {
		return in
	}
	out := append([]RootScope(nil), in...)
	for i := range out {
		out[i].AnalyzedPaths = sortedStrings(out[i].AnalyzedPaths)
		out[i].UnanalyzedPaths = sortedStrings(out[i].UnanalyzedPaths)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Root < out[j].Root
	})
	return out
}

func sortedStrings(in []string) []string {
	if len(in) == 0 {
		return in
	}
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func canonicalCallFacts(in []CallFact) []CallFact {
	if len(in) == 0 {
		return in
	}
	out := append([]CallFact(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].To < out[j].To
	})
	return out
}

func canonicalReachabilityFacts(in []ReachabilityFact) []ReachabilityFact {
	if len(in) == 0 {
		return in
	}
	out := append([]ReachabilityFact(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		if out[i].Sink != out[j].Sink {
			return out[i].Sink < out[j].Sink
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func canonicalCoverage(in Coverage) Coverage {
	if len(in.Diagnostics) == 0 {
		return in
	}
	out := in
	diagnostics := append([]Diagnostic(nil), in.Diagnostics...)
	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].Code != diagnostics[j].Code {
			return diagnostics[i].Code < diagnostics[j].Code
		}
		if diagnostics[i].Path != diagnostics[j].Path {
			return diagnostics[i].Path < diagnostics[j].Path
		}
		return diagnostics[i].Message < diagnostics[j].Message
	})
	out.Diagnostics = diagnostics
	return out
}
