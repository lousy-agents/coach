package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

type sigTestAnalyzeBaselineInterleavedReadFailuresS044185478 struct {
	foundSubjectByPath map[string]string
	sig                codesignal.
				Signal
}

func (sigRecv *sigTestAnalyzeBaselineInterleavedReadFailuresS044185478) call() {

	if sigRecv.sig.RuleID == "state.hidden_input_mutation" {
		sigRecv.foundSubjectByPath[sigRecv.sig.Path] = sigRecv.sig.Subject
	}
}

type sigTestAnalyzeBaselineInterleavedReadFailuresS044248654 struct {
	foundSubjectByPath map[string]string
	path               string
	t                  *testing.
				T
	wantSubject string
}

func (sigRecv *sigTestAnalyzeBaselineInterleavedReadFailuresS044248654) call() {

	if got := sigRecv.foundSubjectByPath[sigRecv.path]; got != sigRecv.wantSubject {
		sigRecv.t.
			Errorf("hidden_input_mutation signal for %q has Subject = %q, want %q (content misaligned across the batch read)", sigRecv.path, got, sigRecv.wantSubject)
	}
}

type sigTestAnalyzeBaselineInterleavedReadFailuresS144847592 struct {
	found   *bool
	missing string
	report  *codesignal.
		Report
}

func (sigRecv *sigTestAnalyzeBaselineInterleavedReadFailuresS144847592) call() {

	for _, d := range sigRecv.report.Diagnostics {
		if d.Path == sigRecv.missing && d.Kind == "head_read_failed" {
			*sigRecv.found = true
		}
	}
}
