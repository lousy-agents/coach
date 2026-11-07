package gitrepo

import (
	"maps"
	"slices"
	"testing"
)

// The set is exactly these fourteen extensions. A kind of file added to it fails the snapshot for a file
// nothing reads, where the snapshot would have written a zero-length file in its place, and
// TestTheFilesTheScanReadsAreExactlyThoseWithTheseExtensions can only name the files it expects the
// snapshot to keep and to drop, not every extension that might be added.
func TestTheSnapshotKeepsExactlyTheExtensionsOfTheFilesTheScanReads(t *testing.T) {
	want := []string{".cjs", ".cts", ".go", ".js", ".json", ".jsonc", ".jsx", ".mjs", ".mod", ".mts", ".sum", ".ts", ".tsx", ".work"}

	got := slices.Sorted(maps.Keys(neededExtensions))

	if !slices.Equal(got, want) {
		t.Errorf("neededExtensions = %v, want %v", got, want)
	}
}
