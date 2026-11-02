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
	if len(layers) == 0 || !model.Coverage.Complete || !everyRootHoldsAPath(filePaths(model.Files), roots) {
		return nil
	}
	dirs := make([]string, 0, len(model.Files))
	for _, file := range model.Files {
		dirs = append(dirs, path.Dir(file.Path))
	}
	membership := domain.NewLayerMembership(dirs, layerPrefixes(layers))
	return &membership
}

// everyRootHoldsAPath reports whether each root has at least one path at or
// under it. A root that holds none was never seen by the analyzer, so the
// files it contains are absent from any count rather than counted inside a
// layer. Go and TypeScript share this rule.
func everyRootHoldsAPath(paths, roots []string) bool {
	if len(paths) == 0 {
		return false
	}
	for _, root := range roots {
		if !rootHoldsAPath(paths, root) {
			return false
		}
	}
	return true
}

func rootHoldsAPath(paths []string, root string) bool {
	for _, p := range paths {
		if rootAllowsDir([]string{root}, p) {
			return true
		}
	}
	return false
}

// tsLayerMembership counts the TypeScript project files under the configured
// roots that no layer prefix contains. It returns nil when the policy declares
// no layers, and whenever the count could understate what sits outside every
// layer:
//   - the model is incomplete, which covers a truncated input budget (the
//     analyzer never saw the skipped tail, so its counts and paths agree and
//     both leave it out) and a root whose candidate files were not all
//     incorporated;
//   - a root analyzed fewer files than it had candidates, even if the model
//     claims completeness;
//   - a root reports an analyzed-file count that its path list does not
//     match: the analyzer then sent counts without paths, and counting from
//     the paths alone would report a false zero;
//   - a configured root has no analyzed file at or under it from any root,
//     which is what a root without a tsconfig and outside every other root
//     reports. A nested root with no tsconfig of its own is counted, because
//     the root above it analyzed the files under it.
//
// Go makes the same decisions in GoLayerMembership.
func tsLayerMembership(coverage Coverage, scopes []RootScope, analyzedPaths []string, layers []ProjectScopePolicyLayer) *domain.LayerMembership {
	if len(layers) == 0 || !coverage.Complete {
		return nil
	}
	roots := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		if scope.AnalyzedFiles < scope.CandidateFiles || len(scope.AnalyzedPaths) != scope.AnalyzedFiles {
			return nil
		}
		roots = append(roots, scope.Root)
	}
	if !everyRootHoldsAPath(analyzedPaths, roots) {
		return nil
	}
	membership := domain.NewLayerMembership(analyzedPaths, layerPrefixes(layers))
	return &membership
}

func filePaths(files []File) []string {
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	return paths
}

func layerPrefixes(layers []ProjectScopePolicyLayer) []string {
	var prefixes []string
	for _, layer := range layers {
		prefixes = append(prefixes, layer.Prefixes...)
	}
	return prefixes
}
