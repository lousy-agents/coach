package domain

import (
	"encoding/json"
	"sort"
)

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
func sortedStrings(in []string) []string {
	if len(in) == 0 {
		return in
	}
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
