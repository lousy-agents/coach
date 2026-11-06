package projectconfig

import (
	"os"
	"path/filepath"
	"testing"
)

// The policies this repository keeps for scanning itself are read by the tool they
// configure, so a schema change that leaves one unreadable would go unnoticed until
// someone scanned with it. The TypeScript semantics policy is named only in the
// conventions doc, and nothing else reads it, so this is the only place that would
// notice.
func TestTheRepositorysOwnPoliciesAreReadableByTheSchema(t *testing.T) {
	for _, name := range []string{"coach.json", "coach.typescript-js-semantics.json"} {
		t.Run(name, func(t *testing.T) { requireThePolicyParses(t, name) })
	}
}

func requireThePolicyParses(t *testing.T, name string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", ".lousy-agents", name))
	if err != nil {
		t.Fatalf("reading the repository's policy: %v", err)
	}

	config, err := Parse(data)

	if err != nil || len(config.Roots) == 0 {
		t.Errorf("Parse(%s) = %d roots, %v; the policy must satisfy the schema it configures and name roots", name, len(config.Roots), err)
	}
}
