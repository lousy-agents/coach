package projectmodel

import (
	"io/fs"
	"path"
	"strings"

	"github.com/lousy-agents/coach/pkg/domain"
)

// layerAmbiguity says why a required layer could not be applied. unmatched is
// the cause the user can fix by correcting a prefix: no package or file sits
// under the layer's prefixes.
type layerAmbiguity struct {
	ambiguous bool
	unmatched bool
}

// diagnostics adds to base the diagnostics for the required layer: the
// ambiguous-layer code whenever the layer could not be applied or a reached
// node lies outside every layer, and the unmatched-layer diagnostic only for
// a layer under which nothing sits.
func (a layerAmbiguity) diagnostics(base []Diagnostic, layer BypassLayer, unclassifiedNodeSeen bool) []Diagnostic {
	diagnostics := append([]Diagnostic{}, base...)
	if a.ambiguous {
		diagnostics = append(diagnostics, Diagnostic{Code: DiagLayerBypassAmbiguousLayer})
		if a.unmatched {
			diagnostics = append(diagnostics, requiredLayerUnmatchedDiagnostic(layer))
		}
	} else if unclassifiedNodeSeen && !containsDiagnosticCode(diagnostics, DiagLayerBypassAmbiguousLayer) {
		diagnostics = append(diagnostics, Diagnostic{Code: DiagLayerBypassAmbiguousLayer})
	}
	return diagnostics
}

// goLayerHasPackage reports whether a Go package with a non-test source file
// sits under layer's prefixes in snapshot, whether or not it declares a function.
func goLayerHasPackage(snapshot fs.FS, layer BypassLayer) bool {
	found := false
	_ = fs.WalkDir(snapshot, ".", func(name string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return nil
		case entry.IsDir():
			return skipDirectory(name, entry)
		case found:
			return fs.SkipAll
		}
		found = isGoSource(name) && domain.AnyLayerPrefixContains(layer.Prefixes, path.Dir(name))
		return nil
	})
	return found
}

func skipDirectory(name string, entry fs.DirEntry) error {
	base := entry.Name()
	if name != "." && (strings.HasPrefix(base, ".") || base == "vendor" || base == "testdata") {
		return fs.SkipDir
	}
	return nil
}

func isGoSource(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}
