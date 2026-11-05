package codesignalcli

import (
	"strings"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// droppedPaths lists the paths the overlay could not serve: a symbolic link, a
// file it could not read, and a file that changed while it was read. Project
// analysis ran without them.
func (v *WorktreeView) droppedPaths() []string {
	dropped := append([]string(nil), v.outcome.SymlinkSkipped...)
	dropped = append(dropped, unreadablePaths(v.outcome.Unreadable)...)
	return append(dropped, v.outcome.Changed...)
}

// coverageAfterDrops marks head coverage incomplete when a dropped file lies
// inside the project's roots: the model was built from fewer files than the
// project holds, so it must not report itself complete.
func (v *WorktreeView) coverageAfterDrops(coverage *projectmodel.Coverage, roots []string) *projectmodel.Coverage {
	if v == nil || coverage == nil || !coverage.Complete {
		return coverage
	}
	for _, name := range v.droppedPaths() {
		if underAnyRoot(name, roots) {
			incomplete := *coverage
			incomplete.Complete = false
			return &incomplete
		}
	}
	return coverage
}

func underAnyRoot(name string, roots []string) bool {
	if len(roots) == 0 {
		return true
	}
	for _, root := range roots {
		if root == "" || root == "." || name == root || strings.HasPrefix(name, strings.TrimSuffix(root, "/")+"/") {
			return true
		}
	}
	return false
}
