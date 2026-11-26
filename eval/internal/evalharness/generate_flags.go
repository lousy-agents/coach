package evalharness

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

const unset = -1

// GenerateRequest is a parsed `coach-eval generate` command line.
type GenerateRequest struct {
	Cell    evalgen.Cell
	Seed    int64
	Panel   string
	Out     string
	NoCache bool
}

// Panels name the two cell populations of design §5.
const (
	PanelTuning  = "tuning"
	PanelHeldOut = "held_out"
)

func parseGenerate(args []string, stderr io.Writer) (GenerateRequest, error) {
	flags := flag.NewFlagSet("coach-eval generate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	index := flags.Int("cell", unset, "cell index, 0 to 1727")
	var cell evalgen.Cell
	cell.Layers, cell.Packages, cell.Depth = unset, unset, unset
	flags.IntVar(&cell.Layers, "layers", unset, "layer count, 3 to 5")
	flags.IntVar(&cell.Packages, "packages", unset, "packages per layer, 2 to 4")
	flags.StringVar(&cell.Entry, "entry", "", "entry kind: "+strings.Join(evalgen.EntryKinds, ", "))
	flags.IntVar(&cell.Depth, "depth", unset, "call depth, 2 to 5")
	flags.StringVar(&cell.Field, "field", "", "field type: "+strings.Join(evalgen.FieldTypes, ", "))
	flags.StringVar(&cell.Direction, "direction", "", "direction: "+strings.Join(evalgen.Directions, ", "))
	flags.StringVar(&cell.Trap, "trap", "", "trap: "+strings.Join(evalgen.TrapKinds, ", "))
	request := GenerateRequest{}
	flags.Int64Var(&request.Seed, "seed", 0, "seed")
	flags.StringVar(&request.Panel, "panel", PanelTuning, "tuning or held_out")
	flags.StringVar(&request.Out, "out", "", "copy the program to <out>/<program_id>")
	flags.BoolVar(&request.NoCache, "no-cache", false, "regenerate instead of reading the cache")
	if err := flags.Parse(args); err != nil {
		return request, err
	}
	if request.Panel != PanelTuning && request.Panel != PanelHeldOut {
		return request, fmt.Errorf("panel must be %s or %s, got %q", PanelTuning, PanelHeldOut, request.Panel)
	}
	var err error
	request.Cell, err = chooseCell(*index, cell)
	return request, err
}

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
