package codesignalcli

import (
	"testing"
)

type sigTestPrepareCompilerRemediationOffersOnlyExecutablePrepare struct {
	code string
	got  string
	t    *testing.
		T
}

func (sigRecv *sigTestPrepareCompilerRemediationOffersOnlyExecutablePrepare) call() {

	if sigRecv.got == "" {
		sigRecv.t.
			Errorf("PrepareCompilerRemediation(%q, ...) = \"\", want a non-empty --prepare-compiler command", sigRecv.code)
	}
}
