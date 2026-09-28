package projectmodel

import (
	"fmt"
	"path"
	"strings"
)

type ProjectScopePolicyLayer struct {
	Name     string
	Prefixes []string
}

// ProjectScopePolicy is not pkg/codesignal.LayerPolicy. This package is an
// adapter and must not import the use-case package. Keep this type's Layers
// field shape in sync with ArchitectureLayer by hand if that one changes.
type ProjectScopePolicy struct {
	Roots  []string
	Layers []ProjectScopePolicyLayer
}

func ProjectScopeFromModel(model Model, policy ProjectScopePolicy) (ProjectScope, error) {
	scopeByRoot := make(map[string]RootScope, len(model.RootScopes))
	for _, rs := range model.RootScopes {
		scopeByRoot[path.Clean(rs.Root)] = rs
	}

	roots := make([]ProjectScopeRoot, 0, len(policy.Roots))
	policyScopes := make([]RootScope, 0, len(policy.Roots))
	for _, root := range policy.Roots {
		rootFact, ok := scopeByRoot[path.Clean(root)]
		if !ok {
			return ProjectScope{}, fmt.Errorf("project_scope: policy root %q has no matching root_scopes entry in the analyzer response", root)
		}
		roots = append(roots, ProjectScopeRoot{
			Root:           rootFact.Root,
			CandidateFiles: rootFact.CandidateFiles,
			AnalyzedFiles:  rootFact.AnalyzedFiles,
		})
		policyScopes = append(policyScopes, rootFact)
	}

	analyzedPaths := analyzedFilePathsFromRootScopes(policyScopes)

	matched := make([]string, 0, len(policy.Layers))
	unmatched := make([]string, 0, len(policy.Layers))
	for _, layer := range policy.Layers {
		if layerMatchesAnyPath(layer, analyzedPaths) {
			matched = append(matched, layer.Name)
		} else {
			unmatched = append(unmatched, layer.Name)
		}
	}

	return ProjectScope{
		InclusionRule:   InclusionRuleTSConfigIncludesNoTestClassification,
		PatternSet:      TSReachabilityAlgorithm,
		Roots:           roots,
		MatchedLayers:   matched,
		UnmatchedLayers: unmatched,
	}, nil
}

func analyzedFilePathsFromRootScopes(scopes []RootScope) []string {
	acc := &analyzedPathAccumulator{seen: make(map[string]struct{})}
	for _, scope := range scopes {
		acc.add(scope.AnalyzedPaths)
	}
	return acc.paths
}

type analyzedPathAccumulator struct {
	seen  map[string]struct{}
	paths []string
}

func (a *analyzedPathAccumulator) add(analyzed []string) {
	for _, p := range analyzed {
		if _, exists := a.seen[p]; exists {
			continue
		}
		a.seen[p] = struct{}{}
		a.paths = append(a.paths, p)
	}
}

// layerMatchesAnyPath must stay in sync with pkg/codesignal's matchLayer
// (see ProjectScopePolicy) -- an import cycle prevents sharing one
// implementation.
func layerMatchesAnyPath(layer ProjectScopePolicyLayer, paths []string) bool {
	matched := false
	for _, prefix := range layer.Prefixes {
		matched = matched || prefixMatchesAnyPath(prefix, paths)
	}
	return matched
}

func prefixMatchesAnyPath(prefix string, paths []string) bool {
	for _, path := range paths {
		if prefix == "." || path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
