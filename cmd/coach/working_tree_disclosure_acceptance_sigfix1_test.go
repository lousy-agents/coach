package main

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

type sigworktreeDisclosureBodyS2 struct {
	b *strings.
		Builder
	report *codesignal.
		Report
}

func (sigRecv *sigworktreeDisclosureBodyS2) call() {

	for _, diagnostic := range sigRecv.report.Diagnostics {
		if diagnostic.Kind == codesignal.DiagKindWorktreeChangesNotAnalyzed {
			fmt.Fprintf(sigRecv.b, "%s %s\n", diagnostic.Path, diagnostic.Message)
		}
	}
}
