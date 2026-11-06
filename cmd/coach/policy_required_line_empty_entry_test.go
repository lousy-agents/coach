package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// The refusal of compiler setup says what the scan says about a committed policy whose index holds only
// an empty entry for it: the restore replaces that entry and loses nothing. It builds the sentence with
// the scan's own builder, so the two cannot drift.
func TestPolicyRequiredLineExplainsARestoreOverAnEmptyIndexEntryAsTheScanDoes(t *testing.T) {
	line := policyRequiredLine("coach codesignal", "policy_missing", projectreadiness.RemedyRestoreOverEmptyEntry, "abc123", "pol.json")

	want := projectconfig.RestoreSentence(projectreadiness.RemedyRestoreOverEmptyEntry, "abc123", "pol.json", projectconfig.AsARefusal(projectreadiness.NextActionRestorePolicy))
	if !strings.HasSuffix(line, want) || !strings.Contains(line, "empty entry") || !strings.Contains(line, "restore_policy") {
		t.Errorf("line %q does not end with the shared sentence %q", line, want)
	}
}

// Guided authoring is for a missing policy that a new file fixes. A policy that is restored, with or
// without an empty entry in the way, is not authored again.
func TestOnlyAMissingPolicyAFreshFileFixesIsAuthorable(t *testing.T) {
	cases := map[string]struct {
		remedy projectreadiness.PolicyRemedy
		want   bool
	}{
		"a new policy":                  {projectreadiness.RemedyAuthor, true},
		"a restore":                     {projectreadiness.RemedyRestore, false},
		"a restore over an empty entry": {projectreadiness.RemedyRestoreOverEmptyEntry, false},
		"no remedy":                     {projectreadiness.RemedyNone, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			requireAuthorable(t, &projectconfig.ConfigError{Kind: projectconfig.KindMissing, Remedy: c.remedy}, c.want)
		})
	}
}

// Only a policy missing from disk is authored in a session; a committed policy that is invalid, or not
// found at the revision, has its own message, and an error that is not about a policy has none.
func TestOnlyAPolicyMissingFromDiskIsAuthorable(t *testing.T) {
	cases := map[string]error{
		"an invalid policy that a new document would replace": &projectconfig.ConfigError{Kind: projectconfig.KindInvalid, Remedy: projectreadiness.RemedyAuthor},
		"a policy not found at the revision":                  &projectconfig.ConfigError{Kind: projectconfig.KindNotFound, Remedy: projectreadiness.RemedyAuthor},
		"an error that is not about a policy":                 errors.New("boom"),
	}
	for name, err := range cases {
		t.Run(name, func(t *testing.T) { requireAuthorable(t, err, false) })
	}
}

func requireAuthorable(t *testing.T, err error, want bool) {
	t.Helper()
	if got := authorableMissingPolicy(err); got != want {
		t.Errorf("authorableMissingPolicy(%v) = %v, want %v", err, got, want)
	}
}
