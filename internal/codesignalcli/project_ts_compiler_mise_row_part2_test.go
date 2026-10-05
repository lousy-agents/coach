package codesignalcli

import (
	"testing"
)

func TestMiseInstallCommandIsExactlyTheFrozenTemplate(t *testing.T) {
	got := miseInstallCommand("7.0.2")
	want := "mise install npm:typescript@7.0.2"
	if got != want {
		t.Errorf("miseInstallCommand(%q) = %q, want %q", "7.0.2", got, want)
	}
}

func TestMiseWhereCommandIsExactlyTheFrozenTemplate(t *testing.T) {
	if miseWhereCommand != "mise where" {
		t.Errorf("miseWhereCommand = %q, want %q", miseWhereCommand, "mise where")
	}
}

// TestMiseBackingNpmSuppressionFlagMatchesTheNpmRow pins that the mise
// npm-backend row reuses the npm adapter row's own suppression mechanism
// rather than a mise-specific flag invented separately, since mise installs
// npm:typescript via npm's own resolution.
func TestMiseBackingNpmSuppressionFlagMatchesTheNpmRow(t *testing.T) {
	if miseBackingNpmSuppressionFlag != "--ignore-scripts" {
		t.Errorf("miseBackingNpmSuppressionFlag = %q, want %q", miseBackingNpmSuppressionFlag, "--ignore-scripts")
	}
}
