package configauthoring

import (
	"fmt"
	"io"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func uncoveredDiscoveredDirectories(dirs []string, layers []projectconfig.Layer) []string {
	var uncovered []string
	for _, dir := range dirs {
		if directoryCovered(dir, layers) {
			continue
		}
		uncovered = append(uncovered, dir)
	}
	return uncovered
}

func directoryCovered(dir string, layers []projectconfig.Layer) bool {
	for _, layer := range layers {
		if layerMatchesDirectory(layer, dir) {
			return true
		}
	}
	return false
}

// discoveredDirectories returns discovered.Roots and discovered.Candidates
// combined into one ordered, deduplicated list -- the coverage preview
// treats every directory DiscoverTSRoots found as something the eventual
// policy should account for, not only the ones it called out as tsconfig
// roots.
func discoveredDirectories(discovered projectmodel.TSRootDiscoveryResult) []string {
	acc := &directoryAccumulator{seen: map[string]bool{}}
	acc.add(discovered.Roots)
	acc.add(discovered.Candidates)
	return acc.all
}

type directoryAccumulator struct {
	seen map[string]bool
	all  []string
}

func (a *directoryAccumulator) add(group []string) {
	for _, dir := range group {
		if !a.seen[dir] {
			a.seen[dir] = true
			a.all = append(a.all, dir)
		}
	}
}

func printCoveragePreview(out io.Writer, discovered projectmodel.TSRootDiscoveryResult, layers []projectconfig.Layer) {
	dirs := discoveredDirectories(discovered)

	fmt.Fprintln(out, "Coverage preview:")
	for _, layer := range layers {
		matched := matchingDiscoveredDirectories(layer, dirs)
		fmt.Fprintf(out, "  layer %q (prefixes: %s) matches: %s\n", layer.Name, strings.Join(layer.Prefixes, ", "), formatStringList(matched))
	}

	uncovered := uncoveredDiscoveredDirectories(dirs, layers)
	fmt.Fprintf(out, "  discovered directories no declared layer matches: %s\n", formatStringList(uncovered))
}

func formatStringList(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	return strings.Join(items, ", ")
}
