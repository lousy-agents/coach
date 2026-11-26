package evalharness

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

func chooseCells(tuning bool, index int, dimensions evalgen.Cell, request GenerateRequest) ([]evalgen.Cell, error) {
	panel := request.Panel
	if tuning {
		return tuningCells(index, dimensions, request)
	}
	cell, err := chooseCell(index, dimensions)
	if err != nil {
		return nil, err
	}
	if panel == PanelHeldOut && evalgen.IsTuning(cell) {
		return nil, fmt.Errorf("this cell is one of the twelve tuning cells; the held_out panel never names one")
	}
	return []evalgen.Cell{cell}, nil
}

func tuningCells(index int, dimensions evalgen.Cell, request GenerateRequest) ([]evalgen.Cell, error) {
	if index != unset {
		return nil, fmt.Errorf("--tuning conflicts with --cell: the tuning cells are fixed")
	}
	if given := givenDimensions(dimensions); len(given) > 0 {
		return nil, fmt.Errorf("--tuning conflicts with %s: the tuning cells are fixed", strings.Join(given, ", "))
	}
	if request.Panel == PanelHeldOut {
		return nil, fmt.Errorf("--tuning conflicts with --panel held_out: the tuning cells are never held out")
	}
	return evalgen.TuningCells(), nil
}
