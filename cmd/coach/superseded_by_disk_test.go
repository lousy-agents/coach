package main

import (
	"errors"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func TestARefusalOfTheCommittedCanonicalPolicyGivesWayOnlyToAUsableCanonicalPolicyOnDisk(t *testing.T) {
	canonicalOnDisk := projectconfig.DiskSelection{Path: projectconfig.CanonicalPath}
	legacyOnDisk := projectconfig.DiskSelection{Path: projectconfig.DefaultPath}
	invalid := &projectconfig.ConfigError{Kind: projectconfig.KindInvalid}
	wrongCase := &projectconfig.ConfigError{Kind: projectconfig.KindInvalid, PolicyCause: projectreadiness.CauseNameCaseDiffers}
	tests := []struct {
		name  string
		fatal error
		disk  projectconfig.DiskSelection
		want  bool
	}{
		{"a refusal of the copy, and the canonical policy on disk", invalid, canonicalOnDisk, true},
		{"a refusal of the copy, and only the root project.json on disk", invalid, legacyOnDisk, false},
		{"a refusal of the copy, and nothing on disk", invalid, projectconfig.DiskSelection{}, false},
		{"a policy committed in another case, and the canonical policy usable on disk", wrongCase, canonicalOnDisk, true},
		{"an operational failure, which is no refusal of the copy", errors.New("git: could not run"), canonicalOnDisk, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { expectSuperseded(t, tt.fatal, tt.disk, tt.want) })
	}
}

func expectSuperseded(t *testing.T, fatal error, disk projectconfig.DiskSelection, want bool) {
	t.Helper()
	if got := supersededByDisk(fatal, disk); got != want {
		t.Fatalf("supersededByDisk = %v, want %v", got, want)
	}
}
