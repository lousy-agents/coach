package evalgen_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// refusedShapes are the shapes the base program must not hold anywhere in production code: a branch, a loop, a
// constructor call, a throw, a ternary, a logical operator or a void function (design §6: below every published
// threshold, derived from the rule definitions and not from Coach output).
var refusedShapes = regexp.MustCompile(`\b(if|else|for|while|do|switch|case|try|catch|throw|new|delete|void|let|var)\b|\?|&&|\|\||\.push\(|\.splice\(`)

func isProduction(path string) bool {
	return strings.HasPrefix(path, "src/") && strings.HasSuffix(path, ".ts") && !strings.HasSuffix(path, ".test.ts")
}

// TestEveryTypeScriptCellIsBranchFreeAndDeterministic walks the whole cell space with two seeds, without the
// TypeScript toolchain.
func TestEveryTypeScriptCellIsBranchFreeAndDeterministic(t *testing.T) {
	for index := 0; index < evalgen.CellCount; index++ {
		cell, err := evalgen.CellFromIndex(index)
		if err != nil {
			t.Fatal(err)
		}
		for _, seed := range []int64{0, 3} {
			first, err := evalgen.GenerateTypeScript(cell, seed)
			if err != nil {
				t.Fatalf("cell %d seed %d: %v", index, seed, err)
			}
			second, _ := evalgen.GenerateTypeScript(cell, seed)
			if first.ID != second.ID || !sameTree(first.Tree, second.Tree) {
				t.Fatalf("cell %d seed %d: two generations differ", index, seed)
			}
			assertProduction(t, index, seed, first)
		}
	}
}

func assertProduction(t *testing.T, index int, seed int64, program evalgen.Program) {
	t.Helper()
	for _, files := range []map[string][]byte{program.Tree, program.Solution} {
		for path, source := range files {
			if isProduction(path) && refusedShapes.Match(source) {
				t.Fatalf("cell %d seed %d: %s holds %q", index, seed, path, refusedShapes.Find(source))
			}
		}
	}
	if _, goModule := program.Tree["go.mod"]; goModule {
		t.Fatalf("cell %d seed %d: a TypeScript program holds go.mod", index, seed)
	}
}

func sameTree(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for path, content := range a {
		if !bytes.Equal(content, b[path]) {
			return false
		}
	}
	return true
}
