package projectmodel

import (
	"path"
	"sort"
)

// SnapshotMeta carries revision/config/backend identities the caller
// already resolved (e.g. from Git) needed to populate Model.Snapshot.
// BuildGoModel does not compute these itself.
type SnapshotMeta struct {
	Revision           string
	TreeID             string
	ConfigDigest       string
	BackendDigest      string
	BuildContextDigest string
	Repository         string
}

// selectedRootsFrom cleans and sorts roots for Snapshot.SelectedRoots, so
// callers passing the same roots in a different order still produce
// byte-identical canonical Model JSON. A nil/empty roots yields a nil
// slice, matching Snapshot.SelectedRoots' omitempty contract.
func selectedRootsFrom(roots []string) []string {
	if len(roots) == 0 {
		return nil
	}
	out := make([]string, len(roots))
	for i, r := range roots {
		out[i] = path.Clean(r)
	}
	sort.Strings(out)
	return out
}
