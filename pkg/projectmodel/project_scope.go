package projectmodel

import (
	"fmt"
	"path"
	"strings"
)

const InclusionRuleTSConfigIncludesNoTestClassification = "tsconfig_includes_no_test_classification"

type ProjectScopePolicyLayer struct {
	Name     string
	Prefixes []string
}

// ProjectScopePolicy is not pkg/codesignal.LayerPolicy: pkg/codesignal
// already imports pkg/projectmodel (rule_layer_violation.go), so importing
// pkg/codesignal from here to reuse its type would create an import cycle.
// Keep this type's Layers field shape in sync with ArchitectureLayer by
// hand if that one changes.
type ProjectScopePolicy struct {
	Roots  []string
	Layers []ProjectScopePolicyLayer
}

type ProjectScopeRoot struct {
	Root           string `json:"root"`
	CandidateFiles int    `json:"candidate_files"`
	AnalyzedFiles  int    `json:"analyzed_files"`
}

type ProjectScope struct {
	InclusionRule   string             `json:"inclusion_rule"`
	PatternSet      string             `json:"pattern_set"`
	Roots           []ProjectScopeRoot `json:"roots"`
	MatchedLayers   []string           `json:"matched_layers"`
	UnmatchedLayers []string           `json:"unmatched_layers"`
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
	seen := make(map[string]struct{})
	var paths []string
	for _, scope := range scopes {
		for _, p := range scope.AnalyzedPaths {
			if _, exists := seen[p]; exists {
				continue
			}
			seen[p] = struct{}{}
			paths = append(paths, p)
		}
	}
	return paths
}

// layerMatchesAnyPath must stay in sync with pkg/codesignal's matchLayer
// (see ProjectScopePolicy) -- an import cycle prevents sharing one
// implementation.
func layerMatchesAnyPath(layer ProjectScopePolicyLayer, paths []string) bool {
	for _, prefix := range layer.Prefixes {
		for _, path := range paths {
			if prefix == "." || path == prefix || strings.HasPrefix(path, prefix+"/") {
				return true
			}
		}
	}
	return false
}
