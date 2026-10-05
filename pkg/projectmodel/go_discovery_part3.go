package projectmodel

import (
	"io/fs"
	"path"
	"sort"
	"strings"
)

// handleWalkError applies DiscoverGoRoots' fail-open walk-error policy:
// a failure at the snapshot root is DiagRootUnavailable + SkipAll; any
// other unreadable subtree is skipped so the rest of the walk continues.
func (d *goProjectDiscovery) handleWalkError(p string) error {
	if p == "." {
		d.Complete = false
		d.Diagnostics = append(d.Diagnostics, Diagnostic{Code: DiagRootUnavailable, Path: "."})
		return fs.SkipAll
	}
	return nil
}

// shouldSkipDiscoveryDir reports whether the walk should prune the directory
// at p: testdata/vendor fixtures and dot-prefixed directories (e.g. .git)
// never contain go.mod/go.work files relevant to root discovery.
func shouldSkipDiscoveryDir(p string) bool {
	if p == "." {
		return false
	}
	base := path.Base(p)
	return base == "testdata" || base == "vendor" || strings.HasPrefix(base, ".")
}
func mapKeysSorted[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
