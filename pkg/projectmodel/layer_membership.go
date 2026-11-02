package projectmodel

import (
	"path"

	"github.com/lousy-agents/coach/pkg/domain"
)

// GoLayerMembership counts the Go project files under the configured roots and
// how many sit in a package directory that no layer prefix contains. The Go
// layer rule matches the importing package directory, so a file is outside
// every layer exactly when its directory is. It returns nil when the policy
// declares no layers, because there is then nothing to be outside of. It also
// returns nil when the model is incomplete, because a truncated model's Files
// hold only the analyzed head, and when a configured root holds no model file:
// a root with no module under it (go.mod above the root, say) was never seen,
// so its files are absent from the count rather than counted inside a layer.
func GoLayerMembership(model Model, roots []string, layers []ProjectScopePolicyLayer) *domain.LayerMembership {
	if len(layers) == 0 || !model.Coverage.Complete || !everyRootHoldsAFile(model.Files, roots) {
		return nil
	}
	dirs := make([]string, 0, len(model.Files))
	for _, file := range model.Files {
		dirs = append(dirs, path.Dir(file.Path))
	}
	membership := domain.NewLayerMembership(dirs, layerPrefixes(layers))
	return &membership
}

func everyRootHoldsAFile(files []File, roots []string) bool {
	if len(files) == 0 {
		return false
	}
	for _, root := range roots {
		if !rootHoldsAFile(files, root) {
			return false
		}
	}
	return true
}

func rootHoldsAFile(files []File, root string) bool {
	for _, file := range files {
		if rootAllowsDir([]string{root}, file.Path) {
			return true
		}
	}
	return false
}

// tsLayerMembership counts the TypeScript project files under the configured
// roots that no layer prefix contains. It returns nil when the policy declares
// no layers, and when any policy root reports an analyzed-file count that its
// path list does not match: the analyzer then sent counts without paths, and
// counting from the paths alone would report a false zero. It returns nil when
// a policy root analyzed no file, which is what a root without a tsconfig
// reports: its files were never candidates, so a count over the other roots
// would understate what sits outside every layer. It also returns nil
// when the input budget truncated the files sent to the analyzer. The analyzer
// never saw the skipped tail, so its counts and paths agree and both leave the
// tail out.
func tsLayerMembership(coverage Coverage, scopes []RootScope, analyzedPaths []string, layers []ProjectScopePolicyLayer) *domain.LayerMembership {
	if len(layers) == 0 || containsDiagnosticCode(coverage.Diagnostics, DiagFileBudgetExceeded) {
		return nil
	}
	for _, scope := range scopes {
		if scope.AnalyzedFiles == 0 || len(scope.AnalyzedPaths) != scope.AnalyzedFiles {
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
