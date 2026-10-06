package configauthoring

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/prompt"
)

func layerPrefixAttemptFailed(out io.Writer, reader *bufio.Reader, err error) (retry, cancelled bool) {
	if err == nil {
		return false, false
	}
	if promptRetryOrCancel(out, reader, err.Error()) {
		return false, true
	}
	return true, false
}

func promptLayerPrefixes(out io.Writer, reader *bufio.Reader, name string, existing []projectconfig.Layer) (prefixes []string, cancelled bool) {
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
