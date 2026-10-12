package pkgmanager

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// Contexts resolves the directories Check reads
// manager metadata from: each selected root's nearest package.json, which is
// exactly the context tstoolchain.ResolveCompiler resolves that root's compiler against
// (resolveProjectRoot). A root with no manifest at or above it contributes no
// context; when no root contributes one, the worktree root stands in, so a
// repository whose only metadata is a top-level lockfile beside no
// package.json is still classified rather than silently unchecked --
// fellBack reports when that stand-in fired, so a caller without a
// validated policy can tell that classification apart from one tstoolchain.ResolveCompiler
// actually found a manifest for (Check's own R1 gate).
func Contexts(worktreeRoot string, roots []string) (contexts []string, fellBack bool) {
	if len(roots) == 0 {
		roots = []string{"."}
	}
	contexts = make([]string, 0, len(roots))
	for _, root := range roots {
		if manifestDir, ok := tstoolchain.NearestPackageJSONDir(tstoolchain.SelectedRootAbs(worktreeRoot, root), worktreeRoot); ok {
			contexts = append(contexts, manifestDir)
		}
	}
	if len(contexts) == 0 {
		return []string{worktreeRoot}, true
	}
	return tstoolchain.DedupeStrings(contexts), false
}
