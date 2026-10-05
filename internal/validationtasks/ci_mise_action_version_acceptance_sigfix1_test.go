package validationtasks

import (
	"strings"
)

type sigmiseActionStepBodiesS4 struct {
	end    *int
	indent int
	lines  []string
}

func (sigRecv *sigmiseActionStepBodiesS4) call() {

	for *sigRecv.end < len(sigRecv.lines) {
		if strings.TrimSpace(sigRecv.lines[*sigRecv.end]) == "" {
			*sigRecv.end++
			continue
		}
		lineIndent := len(sigRecv.lines[*sigRecv.end]) - len(strings.TrimLeft(sigRecv.lines[*sigRecv.end], " \t"))
		if lineIndent <= sigRecv.indent {
			break
		}
		*sigRecv.end++
	}
}
