package codesignalcli

import (
	"fmt"
	"io"
	"strings"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func splitTrimmedNonEmpty(s, sep string) []string {
	var result []string
	for _, piece := range strings.Split(s, sep) {
		piece = strings.TrimSpace(piece)
		if piece != "" {
			result = append(result, piece)
		}
	}
	return result
}

func layerMatchesDirectory(layer projectConfigLayer, dir string) bool {
	for _, prefix := range layer.Prefixes {
		if directoryHasPrefix(dir, prefix) {
			return true
		}
	}
	return false
}

func printCoveragePreview(out io.Writer, discovered projectmodel.TSRootDiscoveryResult, layers []projectConfigLayer) {
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
