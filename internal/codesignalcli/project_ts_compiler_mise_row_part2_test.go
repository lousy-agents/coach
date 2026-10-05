package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// TestMiseBackingNpmSuppressionFlagMatchesTheNpmRow pins that the mise
// npm-backend row reuses the npm adapter row's own suppression mechanism
// rather than a mise-specific flag invented separately, since mise installs
// npm:typescript via npm's own resolution.
func TestMiseBackingNpmSuppressionFlagMatchesTheNpmRow(t *testing.T) {
	if tstoolchain.MiseBackingNpmSuppressionFlag != "--ignore-scripts" {
		t.Errorf("miseBackingNpmSuppressionFlag = %q, want %q", tstoolchain.MiseBackingNpmSuppressionFlag, "--ignore-scripts")
	}
}
