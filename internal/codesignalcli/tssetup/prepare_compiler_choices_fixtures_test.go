package tssetup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func writePackageTypescript(t *testing.T, repo, pkg, version string) {
	t.Helper()
	pkgDir := filepath.Join(repo, filepath.FromSlash(pkg))
	tsDir := filepath.Join(pkgDir, "node_modules", "typescript")
	if err := os.MkdirAll(tsDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", tsDir, err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte("{\"name\":\"pkg\",\"version\":\"1.0.0\"}\n"), 0o644); err != nil {
		t.Fatalf("write %s package.json: %v", pkg, err)
	}
	if err := os.WriteFile(filepath.Join(tsDir, "package.json"), []byte("{\"version\":\""+version+"\"}\n"), 0o644); err != nil {
		t.Fatalf("write %s typescript: %v", pkg, err)
	}
}

func verifiedMiseChoiceKinds(choices []projectreadiness.MiseChoice) []string {
	var kinds []string
	for _, choice := range choices {
		if choice.Verified {
			kinds = append(kinds, choice.Kind)
		}
	}
	return kinds
}

func sameChoiceKinds(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
