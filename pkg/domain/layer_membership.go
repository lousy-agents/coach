package domain

import "strings"

// LayerMembership counts a revision's project files under the configured roots
// and how many of them no declared layer contains. It describes reach only:
// code outside every layer is not evaluated by the layer rules, and this says
// how much of it there is. It is information, not a finding.
type LayerMembership struct {
	AnalyzedFiles      int `json:"analyzed_files"`
	FilesOutsideLayers int `json:"files_outside_layers"`
}

// LayerPrefixContains reports whether key lies at or under prefix. The
// prefix "." contains every key. It is the single definition of layer
// membership, shared by the layer rules, the layer-bypass search, config
// authoring, and LayerMembership, so the count agrees with what the rules
// evaluate.
func LayerPrefixContains(prefix, key string) bool {
	return prefix == "." || key == prefix || strings.HasPrefix(key, prefix+"/")
}

// NewLayerMembership counts keys that no prefix contains. A key is the unit
// a layer rule matches: a package directory for Go, a file path for
// TypeScript. Each analyzed file contributes one key.
func NewLayerMembership(keys []string, prefixes []string) LayerMembership {
	outside := 0
	for _, key := range keys {
		if !anyLayerPrefixContains(prefixes, key) {
			outside++
		}
	}
	return LayerMembership{AnalyzedFiles: len(keys), FilesOutsideLayers: outside}
}

func anyLayerPrefixContains(prefixes []string, key string) bool {
	for _, prefix := range prefixes {
		if LayerPrefixContains(prefix, key) {
			return true
		}
	}
	return false
}
