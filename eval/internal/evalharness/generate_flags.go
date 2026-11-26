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
	Language   evalgen.Language
	Cells      []evalgen.Cell
	Seed       int64
	Panel      string
	Out        string
	NoCache    bool
	NoMutation bool
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
	language := flags.String("language", string(evalgen.LanguageGo), "go or typescript")
	flags.Int64Var(&request.Seed, "seed", 0, "seed")
	flags.StringVar(&request.Panel, "panel", PanelTuning, "tuning or held_out")
	flags.StringVar(&request.Out, "out", "", "copy the program to <out>/<program_id>")
	flags.BoolVar(&request.NoCache, "no-cache", false, "regenerate instead of reading the cache")
	flags.BoolVar(&request.NoMutation, "no-mutation", false, "skip the mutation gate (recorded in the manifest; refused for held_out)")
	tuning := flags.Bool("tuning", false, "emit the twelve fixed tuning cells")
	if err := flags.Parse(args); err != nil {
		return request, err
	}
	if request.Panel != PanelTuning && request.Panel != PanelHeldOut {
		return request, fmt.Errorf("panel must be %s or %s, got %q", PanelTuning, PanelHeldOut, request.Panel)
	}
	if request.NoMutation && request.Panel == PanelHeldOut {
		return request, fmt.Errorf("--no-mutation is refused for the %s panel, which is never generated unchecked", PanelHeldOut)
	}
	var err error
	if request.Language, err = evalgen.ParseLanguage(*language); err != nil {
		return request, err
	}
	request.Cells, err = chooseCells(*tuning, *index, cell, request)
	return request, err
}
