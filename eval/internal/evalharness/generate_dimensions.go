package evalharness

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

func chooseCell(index int, dimensions evalgen.Cell) (evalgen.Cell, error) {
	given := givenDimensions(dimensions)
	if index != unset {
		if len(given) > 0 {
			return evalgen.Cell{}, fmt.Errorf("--cell %d conflicts with %s: name the cell by index or by every dimension, not both", index, strings.Join(given, ", "))
		}
		return evalgen.CellFromIndex(index)
	}
	if missing := missingDimensions(dimensions); len(missing) > 0 {
		return evalgen.Cell{}, fmt.Errorf("no cell: missing %s; pass --cell N, or every dimension", strings.Join(missing, ", "))
	}
	return dimensions, dimensions.Validate()
}

type dimensionFlag struct {
	name  string
	given func(evalgen.Cell) bool
}

var dimensionFlags = []dimensionFlag{
	{"--layers", func(c evalgen.Cell) bool { return c.Layers != unset }},
	{"--packages", func(c evalgen.Cell) bool { return c.Packages != unset }},
	{"--entry", func(c evalgen.Cell) bool { return c.Entry != "" }},
	{"--depth", func(c evalgen.Cell) bool { return c.Depth != unset }},
	{"--field", func(c evalgen.Cell) bool { return c.Field != "" }},
	{"--direction", func(c evalgen.Cell) bool { return c.Direction != "" }},
	{"--trap", func(c evalgen.Cell) bool { return c.Trap != "" }},
}

func givenDimensions(cell evalgen.Cell) []string {
	return namesWhere(cell, true)
}

func missingDimensions(cell evalgen.Cell) []string {
	return namesWhere(cell, false)
}

func namesWhere(cell evalgen.Cell, given bool) []string {
	var names []string
	for _, flag := range dimensionFlags {
		if flag.given(cell) == given {
			names = append(names, flag.name)
		}
	}
	return names
}
