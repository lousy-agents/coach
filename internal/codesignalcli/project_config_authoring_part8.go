package codesignalcli

import (
	"bufio"
	"fmt"
	"io"

	"github.com/lousy-agents/coach/internal/codesignalcli/prompt"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

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

func layerPrefixAttemptFailed(out io.Writer, reader *bufio.Reader, err error) (retry, cancelled bool) {
	if err == nil {
		return false, false
	}
	if promptRetryOrCancel(out, reader, err.Error()) {
		return false, true
	}
	return true, false
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

func promptForForbiddenImports(out io.Writer, reader *bufio.Reader, layers []projectConfigLayer) (forbidden []projectForbiddenImport, cancelled bool) {
	fmt.Fprintln(out, "Define forbidden layer-import pairs (a source layer that may not import a destination layer). Leave the source blank to finish.")
	for {
		from, to, done, cancelled := promptForbiddenPair(out, reader, layers, forbidden)
		if cancelled {
			return forbidden, true
		}
		if done {
			return forbidden, false
		}
		forbidden = append(forbidden, projectForbiddenImport{From: from, To: to})
	}
}

func promptLayerPrefixes(out io.Writer, reader *bufio.Reader, name string, existing []projectConfigLayer) (prefixes []string, cancelled bool) {
	for {
		fmt.Fprintf(out, "Enter comma-separated repository-relative path prefixes for layer %q:\n", name)
		fmt.Fprint(out, "> ")
		answer, _ := prompt.ReadLine(reader)
		candidate := splitTrimmedNonEmpty(answer, ",")
		err := validateLayerPrefixCandidate(name, candidate, existing)
		retry, cancelled := layerPrefixAttemptFailed(out, reader, err)
		if cancelled {
			return nil, true
		}
		if retry {
			continue
		}
		return candidate, false
	}
}

// AuthorProjectConfig runs the guided authoring prompts over in/out rather
// than a real terminal. out is the human-facing transcript; candidateOut
// receives only the approved document when outputSet is false -- mixing
// those streams makes a captured candidate unparseable. Collection never
// preselects roots or infers layers.
func AuthorProjectConfig(dir string, in io.Reader, out io.Writer, candidateOut io.Writer, discovered projectmodel.TSRootDiscoveryResult, outputPath string, outputSet bool) AuthoringResult {
	result := collectAuthoringAnswers(in, out, discovered)
	if !result.Approved {
		return result
	}
	return finalizeApprovedCandidate(result, dir, candidateOut, outputPath, outputSet)
}
