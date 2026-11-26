package evalgen_test

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// The TypeScript trees are built from the vocabulary, the route, the policy writer and the hook that the Go generator
// shares, so a change to any of them must change the TypeScript version too, or a cached tree would stay "intact".
func TestTypeScriptVersionIncludesTheSharedGeneratorVersion(t *testing.T) {
	version := evalgen.LanguageTypeScript.Version()

	if !strings.Contains(version, evalgen.GeneratorVersion) {
		t.Fatalf("TypeScript version %q does not carry the shared generator version %q", version, evalgen.GeneratorVersion)
	}
	if !strings.Contains(version, evalgen.TypeScriptGeneratorVersion) {
		t.Fatalf("TypeScript version %q does not carry its own %q", version, evalgen.TypeScriptGeneratorVersion)
	}
	if version == evalgen.LanguageGo.Version() {
		t.Fatalf("both languages have the version %q", version)
	}
}

func TestMethodEntryIsNamedWithItsClassInTheAPISurface(t *testing.T) {
	cell := evalgen.Cell{Layers: 3, Packages: 2, Entry: "method", Depth: 2, Field: "scalar", Direction: "read", Trap: "layering"}

	program, err := evalgen.GenerateTypeScript(cell, 1)
	if err != nil {
		t.Fatal(err)
	}

	entry := program.Manifest.Template.APISurface[0].Entry
	if !strings.Contains(entry, "#Service.get") {
		t.Fatalf("the entry %q names no class", entry)
	}
}
