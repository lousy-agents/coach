package evalharness

import (
	"strings"

	"github.com/lousy-agents/coach/eval/internal/evaldetect"
	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// surfaceSymbols are the functions the manifest's api_surface names, each as the package directory and the name.
func surfaceSymbols(program evalgen.Program) []evaldetect.Symbol {
	prefix := moduleNameOf(program) + "/"
	var symbols []evaldetect.Symbol
	for _, symbol := range program.Manifest.Template.APISurface {
		qualified := strings.TrimPrefix(symbol.Symbol, prefix)
		split := strings.LastIndex(qualified, ".")
		symbols = append(symbols, evaldetect.Symbol{Dir: qualified[:split], Name: qualified[split+1:]})
	}
	return symbols
}

// surfaceEdges are the imports the manifest's route requires: each package of the api surface imports the next, so
// that the entry reaches every hop through the one before it.
func surfaceEdges(program evalgen.Program) []evaldetect.Edge {
	symbols := surfaceSymbols(program)
	var edges []evaldetect.Edge
	for i := 1; i < len(symbols); i++ {
		edges = append(edges, evaldetect.Edge{From: symbols[i-1].Dir, To: symbols[i].Dir})
	}
	return edges
}
