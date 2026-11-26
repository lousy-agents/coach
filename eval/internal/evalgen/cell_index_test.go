package evalgen_test

import (
	"testing"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

func TestCellIndexInvertsCellFromIndex(t *testing.T) {
	for index := 0; index < evalgen.CellCount; index++ {
		cell, err := evalgen.CellFromIndex(index)
		if err != nil {
			t.Fatal(err)
		}
		if got := cell.Index(); got != index {
			t.Fatalf("cell %d has index %d", index, got)
		}
	}
}
