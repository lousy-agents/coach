package projectmodel

import (
	"path"

	"github.com/lousy-agents/coach/pkg/domain"
)

// GoLayerMembership counts the Go project files under the configured roots and
// how many sit in a package directory that no layer prefix contains. The Go
// layer rule matches the importing package directory, so a file is outside
// every layer exactly when its directory is. It returns nil when the policy
// declares no layers, because there is then nothing to be outside of, and when
// the model is incomplete: a truncated model's Files hold only the analyzed
// head, so a count over them would be a false zero for the files the budget
// skipped.
func GoLayerMembership(model Model, layers []ProjectScopePolicyLayer) *domain.LayerMembership {
	if len(layers) == 0 || !model.Coverage.Complete {
		return nil
	}
	dirs := make([]string, 0, len(model.Files))
	for _, file := range model.Files {
		dirs = append(dirs, path.Dir(file.Path))
	}
	membership := domain.NewLayerMembership(dirs, layerPrefixes(layers))
	return &membership
}

// tsLayerMembership counts the TypeScript project files under the configured
// roots that no layer prefix contains. It returns nil when the policy declares
// no layers, and when any policy root reports an analyzed-file count that its
// path list does not match: the analyzer then sent counts without paths, and
// counting from the paths alone would report a false zero. It also returns nil
// when the input budget truncated the files sent to the analyzer. The analyzer
// never saw the skipped tail, so its counts and paths agree and both leave the
// tail out.
func tsLayerMembership(coverage Coverage, scopes []RootScope, analyzedPaths []string, layers []ProjectScopePolicyLayer) *domain.LayerMembership {
	if len(layers) == 0 || containsDiagnosticCode(coverage.Diagnostics, DiagFileBudgetExceeded) {
		return nil
	}
	for _, scope := range scopes {
		if len(scope.AnalyzedPaths) != scope.AnalyzedFiles {
			return nil
		}
	}
	membership := domain.NewLayerMembership(analyzedPaths, layerPrefixes(layers))
	return &membership
}

func layerPrefixes(layers []ProjectScopePolicyLayer) []string {
	var prefixes []string
	for _, layer := range layers {
		prefixes = append(prefixes, layer.Prefixes...)
	}
	return prefixes
}
