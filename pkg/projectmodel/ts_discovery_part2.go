package projectmodel

import (
	"io/fs"
	"path"

	"strings"
)

// handleWalkError applies DiscoverTSRoots' fail-open walk-error policy: a
// failure at the snapshot root is DiagTSRootUnavailable + SkipAll; any other
// unreadable subtree is skipped so the rest of the walk continues.
func (d *tsProjectDiscovery) handleWalkError(p string) error {
	if p == "." {
		d.Complete = false
		d.Diagnostics = append(d.Diagnostics, Diagnostic{Code: DiagTSRootUnavailable, Path: "."})
		return fs.SkipAll
	}
	return nil
}

// shouldSkipTSDiscoveryDir reports whether the walk should prune the
// directory at p: node_modules only ever holds vendored/installed copies,
// never the project's own tsconfig.json/package.json, and dot-prefixed
// directories (e.g. .git) are never TS project roots -- mirroring
// DiscoverGoRoots' vendor/dot-prefixed pruning convention for Go.
func shouldSkipTSDiscoveryDir(p string) bool {
	if p == "." {
		return false
	}
	base := path.Base(p)
	return base == "node_modules" || strings.HasPrefix(base, ".")
}
func (d *tsProjectDiscovery) visitDiscoveryDir(p string) error {
	if shouldSkipTSDiscoveryDir(p) {
		return fs.SkipDir
	}
	return nil
}
