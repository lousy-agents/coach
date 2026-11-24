package main

import (
	"errors"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

func TestOnlyAnOperationalFailureToReadTheCommittedCanonicalPolicyStands(t *testing.T) {
	invalid := &projectconfig.ConfigError{Kind: projectconfig.KindInvalid}
	tests := []struct {
		name  string
		fatal error
		want  bool
	}{
		{"a refusal of the copy", invalid, false},
		{"an operational failure, which is no refusal of the copy", errors.New("git: could not run"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { expectRefusalStanding(t, tt.fatal, tt.want) })
	}
}

func expectRefusalStanding(t *testing.T, fatal error, want bool) {
	t.Helper()
	if got := refusesWhateverDiskHolds(fatal); got != want {
		t.Fatalf("refusesWhateverDiskHolds = %v, want %v", got, want)
	}
}

func TestADiscoveredSourceNamesTheCanonicalPolicyWhenItIsCommittedAndNotUsable(t *testing.T) {
	tests := []struct {
		name      string
		selection projectconfig.Selection
		want      string
	}{
		{"a committed canonical policy that is not usable", projectconfig.Selection{Fatal: errors.New("invalid")}, projectconfig.CanonicalPath},
		{"the committed root project.json", projectconfig.Selection{Path: projectconfig.DefaultPath}, projectconfig.DefaultPath},
		{"nothing committed", projectconfig.Selection{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { expectDiscoveredSource(t, tt.selection, tt.want) })
	}
}

func expectDiscoveredSource(t *testing.T, selection projectconfig.Selection, want string) {
	t.Helper()
	if got := discoveredSource(selection); got.Path != want || got.Explicit {
		t.Fatalf("discoveredSource = %+v, want discovered path %q", got, want)
	}
}
