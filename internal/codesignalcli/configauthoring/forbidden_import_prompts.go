package configauthoring

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/prompt"
)

func promptForbiddenPair(out io.Writer, reader *bufio.Reader, layers []projectconfig.Layer, existing []projectconfig.ForbiddenImport) (from, to string, done, cancelled bool) {
	for {
		fmt.Fprintln(out, "Enter the source layer name for a forbidden import pair, or leave blank to finish:")
		fmt.Fprint(out, "> ")
		fromAnswer, _ := prompt.ReadLine(reader)
		fromAnswer = strings.TrimSpace(fromAnswer)
		if fromAnswer == "" {
			return "", "", true, false
		}

		fmt.Fprintln(out, "Enter the destination layer name that the source layer may not import:")
		fmt.Fprint(out, "> ")
		toAnswer, _ := prompt.ReadLine(reader)
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

func forbiddenPairCancelled(out io.Writer, reader *bufio.Reader, err error) bool {
	if err == nil {
		return false
	}
	return promptRetryOrCancel(out, reader, err.Error())
}

func promptForForbiddenImports(out io.Writer, reader *bufio.Reader, layers []projectconfig.Layer) (forbidden []projectconfig.ForbiddenImport, cancelled bool) {
	fmt.Fprintln(out, "Define forbidden layer-import pairs (a source layer that may not import a destination layer). Leave the source blank to finish.")
	for {
		from, to, done, cancelled := promptForbiddenPair(out, reader, layers, forbidden)
		if cancelled {
			return forbidden, true
		}
		if done {
			return forbidden, false
		}
		forbidden = append(forbidden, projectconfig.ForbiddenImport{From: from, To: to})
	}
}
