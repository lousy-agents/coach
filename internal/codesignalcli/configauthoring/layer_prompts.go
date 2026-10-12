package configauthoring

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/prompt"
)

// promptForLayers collects named layers and their prefixes, one at a time,
// until the user leaves a layer name blank. It never suggests a name or a
// prefix on the user's behalf: every layer in the returned slice came from
// the user's own typed answers. Each candidate is checked with the same
// name-uniqueness and prefix-overlap rules the frozen project-config schema
// itself enforces (validateLayers), so a mistake is caught and
// explained here rather than deferred to a later validation pass.
func promptForLayers(out io.Writer, reader *bufio.Reader) (layers []projectconfig.Layer, cancelled bool) {
	fmt.Fprintln(out, "Define named layers for architecture-boundary policy. Each layer needs a name and one or more repository-relative path prefixes.")
	for {
		name, done, cancelled := promptLayerName(out, reader, layers)
		if cancelled {
			return layers, true
		}
		if done {
			return layers, false
		}

		prefixes, cancelled := promptLayerPrefixes(out, reader, name, layers)
		if cancelled {
			return layers, true
		}
		layers = append(layers, projectconfig.Layer{Name: name, Prefixes: prefixes})
	}
}

func promptLayerName(out io.Writer, reader *bufio.Reader, existing []projectconfig.Layer) (name string, done, cancelled bool) {
	for {
		fmt.Fprintln(out, "Enter a layer name, or leave blank to finish defining layers:")
		fmt.Fprint(out, "> ")
		answer, _ := prompt.ReadLine(reader)
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

func layerNameDeclared(name string, layers []projectconfig.Layer) bool {
	for _, layer := range layers {
		if layer.Name == name {
			return true
		}
	}
	return false
}
