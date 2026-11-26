package evalgen_test

import (
	"testing"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// identityCell and the values below pin which versions feed the TypeScript cache key and the TypeScript stream seed.
// A change to anything the Go generator shares (vocabulary, route, policy writer, hook) bumps evalgen.GeneratorVersion,
// and must then change both values; a TypeScript-only change bumps evalgen.TypeScriptGeneratorVersion and does the
// same. Update them together with the version, and read the diff of the trees.
var identityCell = evalgen.Cell{Layers: 4, Packages: 3, Entry: "method", Depth: 3, Field: "nested", Direction: "write", Trap: "complexity"}

const (
	pinnedTypeScriptKey = "cca1af5c2604de6e68d1f29def4e097ec12d4c6ffc387ce264b91cec6a70a41b"
	pinnedGoKey         = "882cc4c8065b91e002eaeb55861088f8a8c4dc8569b3bda85a29b0f907dce5b1"
	pinnedProgramID     = "c536499dea3163c9a1bdd7238d1c340fc454274d2f9595d774fcb0038ccdee26"
)

func TestTheCacheKeysOfBothLanguagesArePinned(t *testing.T) {
	if got := evalgen.Key(evalgen.LanguageTypeScript, identityCell, 3, "tuning"); got != pinnedTypeScriptKey {
		t.Errorf("TypeScript key = %s, want %s", got, pinnedTypeScriptKey)
	}
	if got := evalgen.Key(evalgen.LanguageGo, identityCell, 3, "tuning"); got != pinnedGoKey {
		t.Errorf("Go key = %s, want %s", got, pinnedGoKey)
	}
}

func TestTheTypeScriptStreamIsSeededWithBothGeneratorVersions(t *testing.T) {
	program, err := evalgen.GenerateTypeScript(identityCell, 3)
	if err != nil {
		t.Fatal(err)
	}

	if program.ID != pinnedProgramID {
		t.Errorf("program_id = %s, want %s", program.ID, pinnedProgramID)
	}
}
