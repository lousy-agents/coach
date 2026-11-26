package evalgen_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

var jsxFile = regexp.MustCompile(`\.[jt]sx$`)

// slashOutsideStrings finds a `/` that is not inside a string: the layer grader refuses a slash it cannot classify,
// so the generated sources hold none, which also rules out regular expressions and divisions.
var doubleQuoted = regexp.MustCompile(`"(?:[^"\\\n]|\\.)*"`)

func slashOutsideStrings(source []byte) bool {
	return bytes.Contains(doubleQuoted.ReplaceAll(source, nil), []byte("/"))
}

func assertGraderSubset(t *testing.T, index int, seed int64, program evalgen.Program) {
	t.Helper()
	for _, files := range []map[string][]byte{program.Tree, program.Tests, program.Solution} {
		for path, source := range files {
			assertFileInGraderSubset(t, index, seed, path, source)
		}
	}
}

func assertFileInGraderSubset(t *testing.T, index int, seed int64, path string, source []byte) {
	t.Helper()
	if jsxFile.MatchString(path) {
		t.Fatalf("cell %d seed %d: %s is JSX", index, seed, path)
	}
	if strings.HasSuffix(path, ".ts") && slashOutsideStrings(source) {
		t.Fatalf("cell %d seed %d: %s holds a slash outside a string", index, seed, path)
	}
}
