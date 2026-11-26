package evalgen_test

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// The generated base has to stay below every Coach threshold by construction, so no entry kind, direction or
// field type may add a branch, a constructor or a pointer return.
func TestEveryCellEmitsBranchFreeProductionCodeWithoutPointerReturns(t *testing.T) {
	for _, run := range cellRuns() {
		for _, violation := range violationsOfCell(t, run.index, run.seed) {
			t.Errorf("cell %d seed %d: %s", run.index, run.seed, violation)
		}
	}
}

type cellRun struct {
	index int
	seed  int64
}

func cellRuns() []cellRun {
	runs := make([]cellRun, 0, 2*evalgen.CellCount)
	for index := 0; index < evalgen.CellCount; index++ {
		runs = append(runs, cellRun{index, 0}, cellRun{index, 3})
	}
	return runs
}

func violationsOfCell(t *testing.T, index int, seed int64) []string {
	t.Helper()
	cell, err := evalgen.CellFromIndex(index)
	if err != nil {
		t.Fatal(err)
	}
	program, err := evalgen.Generate(cell, seed)
	if err != nil {
		t.Fatalf("cell %d seed %d: %v", index, seed, err)
	}
	var violations []string
	for path, content := range program.Tree {
		violations = append(violations, productionViolations(t, path, content)...)
	}
	return violations
}

func productionViolations(t *testing.T, path string, content []byte) []string {
	t.Helper()
	if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
		return nil
	}
	var violations []string
	for _, violation := range shapeViolations(t, path, content) {
		violations = append(violations, path+": "+violation)
	}
	return violations
}
