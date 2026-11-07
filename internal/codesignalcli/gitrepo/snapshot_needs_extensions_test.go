package gitrepo

import "testing"

// The files an analyzer reads are written first, and the snapshot fails only when those alone pass its total.
// A name left out of the set is written as a zero-length file past the total, so the scan reads nothing of it and
// says nothing, and every extension of the set has to be named for that to be seen.
func TestTheFilesTheScanReadsAreExactlyThoseWithTheseExtensions(t *testing.T) {
	for _, name := range []string{"a.go", "go.mod", "go.sum", "go.work", "a.ts", "a.tsx", "a.mts", "a.cts", "a.js", "a.jsx", "a.mjs", "a.cjs", "a.json", "a.jsonc", "A.GO", "dir/B.TS"} {
		if !needsContent(name) {
			t.Errorf("needsContent(%q) = false; the scan reads it, so it is written before the others and fails the snapshot when it does not fit", name)
		}
	}
	for _, name := range []string{"a.txt", "a.png", "README.md", "Makefile", "a.go.bak", "a.golang", "a.tsx.map"} {
		if needsContent(name) {
			t.Errorf("needsContent(%q) = true; no analyzer reads it", name)
		}
	}
}
