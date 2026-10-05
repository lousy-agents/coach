package projectmodel

import (
	"os"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
)

// TestCallGraphAlgorithmVersionMatchesGoMod guards CallGraphAlgorithm's
// pinned golang.org/x/tools version suffix against drifting from go.mod
// (which renovate.json auto-merges), so the provenance string it embeds in
// every CallGraphResult stays accurate.
func TestCallGraphAlgorithmVersionMatchesGoMod(t *testing.T) {
	data, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatalf("reading repository go.mod: %v", err)
	}
	mf, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		t.Fatalf("parsing repository go.mod: %v", err)
	}

	var pinnedVersion string
	for _, req := range mf.Require {
		if req.Mod.Path == "golang.org/x/tools" {
			pinnedVersion = req.Mod.Version
			break
		}
	}
	if pinnedVersion == "" {
		t.Fatal("golang.org/x/tools requirement not found in go.mod")
	}

	want := "golang.org/x/tools@" + pinnedVersion
	if !strings.HasSuffix(CallGraphAlgorithm, want) {
		t.Errorf("CallGraphAlgorithm %q does not reflect go.mod's pinned %q; update the const when x/tools is bumped", CallGraphAlgorithm, want)
	}
}
