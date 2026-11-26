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
	pinnedTypeScriptKey = "ca71b0d5f56b936495f9680799387fa41c696912255ba440e0749de6ab3fef50"
	pinnedGoKey         = "ec9345c7dc6ea871d95ada7be2af21e49562283f389e93b0d1aafdb8acf55f01"
	pinnedProgramID     = "b9b810426b06d3e783724d79210feee8e3e07b6d4814c8e3e4d93d2af6ddd950"
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
