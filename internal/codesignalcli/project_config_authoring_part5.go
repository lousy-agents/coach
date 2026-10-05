package codesignalcli

import (
	"bufio"

	"fmt"
	"io"

	"github.com/lousy-agents/coach/pkg/projectmodel"

	"strings"
)

func promptLayerName(out io.Writer, reader *bufio.Reader, existing []projectConfigLayer) (name string, done, cancelled bool) {
	for {
		fmt.Fprintln(out, "Enter a layer name, or leave blank to finish defining layers:")
		fmt.Fprint(out, "> ")
		answer, _ := readLine(reader)
		answer = strings.TrimSpace(answer)
		if answer == "" {
			return "", true, false
		}
		declared := layerNameDeclared(answer, existing)
		if declared && promptRetryOrCancel(out, reader, fmt.Sprintf("layer name %q is already used", answer)) {
			return "", false, true
		}
		if declared {
			continue
		}

		return answer, false, false
	}
}
func collectAuthoringAnswers(in io.Reader, transcript io.Writer, discovered projectmodel.TSRootDiscoveryResult) AuthoringResult {
	reader := bufio.NewReader(in)
	roots, cancelled := promptForRoots(transcript, reader, discovered)
	if cancelled {
		return AuthoringResult{Cancelled: true}
	}

	layers, cancelled := promptForLayers(transcript, reader)
	if cancelled {
		return AuthoringResult{Roots: roots, Layers: layers, Cancelled: true}
	}

	forbidden, cancelled := promptForForbiddenImports(transcript, reader, layers)
	if cancelled {
		return AuthoringResult{Roots: roots, Layers: layers, ForbiddenImports: forbidden, Cancelled: true}
	}

	requiredLayer, cancelled := promptForRequiredLayer(transcript, reader, layers)
	if cancelled {
		return AuthoringResult{Roots: roots, Layers: layers, ForbiddenImports: forbidden, Cancelled: true}
	}

	approved := promptForApproval(transcript, reader, discovered, roots, layers, forbidden, requiredLayer)
	return AuthoringResult{Roots: roots, Layers: layers, ForbiddenImports: forbidden, RequiredLayer: requiredLayer, Approved: approved}
}
func matchingDiscoveredDirectories(layer projectConfigLayer, dirs []string) []string {
	var matched []string
	for _, dir := range dirs {
		if layerMatchesDirectory(layer, dir) {
			matched = append(matched, dir)
		}
	}
	return matched
}
