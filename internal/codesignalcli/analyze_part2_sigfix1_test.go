package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

type sigTestAnalyzeBaselineS044942786 struct {
	d codesignal.
		Diagnostic
	foundHeadReadFailed *bool
}

func (sigRecv *sigTestAnalyzeBaselineS044942786) call() {

	if sigRecv.d.Path == "missing.go" && sigRecv.d.Kind == "head_read_failed" {
		*sigRecv.foundHeadReadFailed = true
	}
}

type sigTestAnalyzeBaselineS044059074 struct {
	d codesignal.
		Diagnostic
	foundSyntaxErrors *bool
}

func (sigRecv *sigTestAnalyzeBaselineS044059074) call() {

	if sigRecv.d.Path == "broken.go" && sigRecv.d.Kind == "syntax_errors" {
		*sigRecv.foundSyntaxErrors = true
	}
}

type sigTestAnalyzeBaselineS044484681 struct {
	foundBaselineSignal *bool
	sig                 codesignal.
				Signal
	t *testing.
		T
}

func (sigRecv *sigTestAnalyzeBaselineS044484681) call() {

	if sigRecv.sig.Path == "clean.go" {
		if sigRecv.sig.Lifecycle != "baseline" {
			sigRecv.t.
				Errorf("signal for clean.go has Lifecycle = %q, want %q", sigRecv.sig.Lifecycle, "baseline")
		}
		*sigRecv.foundBaselineSignal = true
	}
}
