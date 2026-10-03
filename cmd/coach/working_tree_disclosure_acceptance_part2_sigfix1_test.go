package main

import (
	"github.com/lousy-agents/coach/pkg/codesignal"
)

type sigexpectStatusCheckFailureDiagnosticS3 struct {
	found  *bool
	report *codesignal.
		Report
}

func (sigRecv *sigexpectStatusCheckFailureDiagnosticS3) call() {

	for _, diagnostic := range sigRecv.report.Diagnostics {
		if recordsStatusCheckFailure(diagnostic.Kind, diagnostic.Message) {
			*sigRecv.found = true
		}
	}
}
