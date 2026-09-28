package codesignalcli

import (
	"bufio"
	"encoding/json"
	"fmt"

	"io"

	"strings"
)

func promptForbiddenPair(out io.Writer, reader *bufio.Reader, layers []projectConfigLayer, existing []projectForbiddenImport) (from, to string, done, cancelled bool) {
	for {
		fmt.Fprintln(out, "Enter the source layer name for a forbidden import pair, or leave blank to finish:")
		fmt.Fprint(out, "> ")
		fromAnswer, _ := readLine(reader)
		fromAnswer = strings.TrimSpace(fromAnswer)
		if fromAnswer == "" {
			return "", "", true, false
		}

		fmt.Fprintln(out, "Enter the destination layer name that the source layer may not import:")
		fmt.Fprint(out, "> ")
		toAnswer, _ := readLine(reader)
		toAnswer = strings.TrimSpace(toAnswer)

		err := validateForbiddenPairCandidate(fromAnswer, toAnswer, layers, existing)
		if forbiddenPairCancelled(out, reader, err) {
			return "", "", false, true
		}
		if err != nil {
			continue
		}
		return fromAnswer, toAnswer, false, false
	}
}
func uncoveredDiscoveredDirectories(dirs []string, layers []projectConfigLayer) []string {
	var uncovered []string
	for _, dir := range dirs {
		if directoryCovered(dir, layers) {
			continue
		}
		uncovered = append(uncovered, dir)
	}
	return uncovered
}

func directoryCovered(dir string, layers []projectConfigLayer) bool {
	for _, layer := range layers {
		if layerMatchesDirectory(layer, dir) {
			return true
		}
	}
	return false
}

func forbiddenPairCancelled(out io.Writer, reader *bufio.Reader, err error) bool {
	if err == nil {
		return false
	}
	return promptRetryOrCancel(out, reader, err.Error())
}

// buildApprovedCandidate renders the collected fields as the schema-1
// project-config document and runs it through parseProjectConfig -- the same
// validator LoadProjectConfig applies to a committed --project-config file
// -- before treating it as valid. source_sink_pack is never populated: it is
// a reserved field this feature must not touch.
func buildApprovedCandidate(roots []string, layers []projectConfigLayer, forbidden []projectForbiddenImport, requiredLayer string) ([]byte, error) {
	candidate := projectConfig{
		SchemaVersion:    "1",
		Roots:            roots,
		Layers:           layers,
		ForbiddenImports: forbidden,
		RequiredLayer:    requiredLayer,
	}
	data, err := json.MarshalIndent(candidate, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	if _, err := parseProjectConfig(data); err != nil {
		return nil, err
	}
	return data, nil
}
