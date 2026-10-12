package configauthoring

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/prompt"
)

func promptForRequiredLayer(out io.Writer, reader *bufio.Reader, layers []projectconfig.Layer) (requiredLayer string, cancelled bool) {
	for {
		fmt.Fprintln(out, "Enter the name of a required intermediary layer, or leave blank for none:")
		fmt.Fprint(out, "> ")
		answer, _ := prompt.ReadLine(reader)
		answer = strings.TrimSpace(answer)
		if answer == "" {
			return "", false
		}
		known := layerNameDeclared(answer, layers)
		if !known && promptRetryOrCancel(out, reader, fmt.Sprintf("required_layer references undefined layer %q", answer)) {
			return "", true
		}
		if !known {
			continue
		}

		return answer, false
	}
}
