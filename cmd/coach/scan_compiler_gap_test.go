package main

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

func TestScanShouldOfferCompilerSetupIgnoresRuntimeUnresolvedError(t *testing.T) {
	err := &tstoolchain.RuntimeUnresolvedError{Code: projectreadiness.GapNodeMissing, ConfigPath: "project.json"}
	if wrapped, ok := scanShouldOfferCompilerSetup(err, false); ok {
		t.Fatalf("scanShouldOfferCompilerSetup(RuntimeUnresolvedError) = (%v, true), want false: a runtime gap must not enter the compiler-setup offer", wrapped)
	}
}
