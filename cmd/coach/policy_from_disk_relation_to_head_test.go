package main

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func TestChoosePolicyRelatesThePolicyOnDiskToWhatHeadHoldsAtItsPath(t *testing.T) {
	readable := func(content string) projectconfig.CommittedCopy {
		return projectconfig.CommittedCopy{Kind: projectconfig.CopyReadable, Content: []byte(content)}
	}
	tests := []struct {
		name      string
		head      projectconfig.CommittedCopy
		converted bool
		want      codesignal.PolicyCause
	}{
		{"nothing at HEAD", projectconfig.CommittedCopy{Kind: projectconfig.CopyAbsent}, false, codesignal.PolicyCauseNotCommitted},
		{"a blob HEAD holds that cannot be read", projectconfig.CommittedCopy{Kind: projectconfig.CopyUnreadable}, false, codesignal.PolicyCauseHeadUnreadable},
		{"a link, directory or submodule", projectconfig.CommittedCopy{Kind: projectconfig.CopyNotAFile}, false, codesignal.PolicyCauseDiffersFromHead},
		{"the same bytes", readable(`{"a":1}`), false, codesignal.PolicyCauseSameAsHead},
		{"the same bytes in another line-ending convention", readable("{\"a\":\r\n1}"), true, codesignal.PolicyCauseSameAsHead},
		{"other bytes", readable(`{"a":2}`), false, codesignal.PolicyCauseDiffersFromHead},
		{"bytes that are not a policy", readable(`{bad`), false, codesignal.PolicyCauseDiffersFromHead},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { expectRelationToHead(t, tt.head, tt.converted, tt.want) })
	}
}

func expectRelationToHead(t *testing.T, head projectconfig.CommittedCopy, converted bool, want codesignal.PolicyCause) {
	t.Helper()
	content := []byte(`{"a":1}`)
	disk := projectconfig.DiskSelection{Path: projectconfig.CanonicalPath, Digest: projectconfig.Digest(content), Content: content}

	scope := choosePolicy(policyEvidence{headPath: disk.Path, disk: disk, head: head, holdsConvertedAt: func(string) bool { return converted }}).scope

	wantDigest := disk.Digest
	if want == codesignal.PolicyCauseSameAsHead {
		wantDigest = projectconfig.Digest(head.Content)
	}
	if scope.Cause != want || scope.Path != disk.Path || scope.Digest != wantDigest || scope.MatchesHead != (want == codesignal.PolicyCauseSameAsHead) {
		t.Fatalf("scope = %+v, want cause %q at the path on disk with digest %q", scope, want, wantDigest)
	}
}

func TestChoosePolicyUsesNoPolicyWhereTheDiskHasNone(t *testing.T) {
	choice := choosePolicy(policyEvidence{headPath: projectconfig.CanonicalPath})

	if choice.scope.Path != "" || len(choice.content) != 0 {
		t.Fatalf("choice = %+v, want no policy: no committed copy stands in for a policy missing on disk", choice)
	}
}

func TestChoosePolicyRelatesTheRootProjectJSONOnDiskToTheCanonicalPolicyHeadSelects(t *testing.T) {
	content := []byte(`{"a":1}`)
	disk := projectconfig.DiskSelection{Path: projectconfig.DefaultPath, Digest: projectconfig.Digest(content), Content: content}
	tests := []struct {
		name string
		head projectconfig.CommittedCopy
		want codesignal.PolicyCause
	}{
		{"a root project.json HEAD commits with the same bytes is not the policy HEAD selects", projectconfig.CommittedCopy{Kind: projectconfig.CopyReadable, Content: content}, codesignal.PolicyCauseDiffersFromHead},
		{"a root project.json HEAD commits with other bytes", projectconfig.CommittedCopy{Kind: projectconfig.CopyReadable, Content: []byte(`{"a":2}`)}, codesignal.PolicyCauseDiffersFromHead},
		{"a root project.json HEAD does not commit", projectconfig.CommittedCopy{Kind: projectconfig.CopyAbsent}, codesignal.PolicyCauseNotCommitted},
		{"a root project.json HEAD holds as a blob that cannot be read", projectconfig.CommittedCopy{Kind: projectconfig.CopyUnreadable}, codesignal.PolicyCauseHeadUnreadable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { expectCauseOfRootProjectJSON(t, disk, tt.head, tt.want) })
	}
}

func expectCauseOfRootProjectJSON(t *testing.T, disk projectconfig.DiskSelection, head projectconfig.CommittedCopy, want codesignal.PolicyCause) {
	t.Helper()
	evidence := policyEvidence{headPath: projectconfig.CanonicalPath, disk: disk, head: head, holdsConvertedAt: func(string) bool { return true }}

	if cause := choosePolicy(evidence).scope.Cause; cause != want {
		t.Fatalf("cause = %q, want %q", cause, want)
	}
}

func TestHeadBoundOfNamesTheBoundOfGitThatStoppedTheReadOfHeadsCopy(t *testing.T) {
	tests := []struct {
		name  string
		bound projectconfig.CopyBound
		want  codesignal.HeadBound
	}{
		{"no bound", projectconfig.CopyBoundNone, codesignal.HeadBoundNone},
		{"git's time bound", projectconfig.CopyBoundTime, codesignal.HeadBoundTime},
		{"git's stderr bound", projectconfig.CopyBoundStderr, codesignal.HeadBoundStderr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { expectHeadBound(t, tt.bound, tt.want) })
	}
}

func expectHeadBound(t *testing.T, bound projectconfig.CopyBound, want codesignal.HeadBound) {
	t.Helper()
	content := []byte(`{"a":1}`)
	disk := projectconfig.DiskSelection{Path: projectconfig.CanonicalPath, Digest: projectconfig.Digest(content), Content: content}
	head := projectconfig.CommittedCopy{Kind: projectconfig.CopyUnreadable, Bound: bound}

	scope := choosePolicy(policyEvidence{disk: disk, head: head}).scope

	if scope.Cause != codesignal.PolicyCauseHeadUnreadable || scope.HeadBound != want {
		t.Fatalf("scope = %+v, want head_unreadable with bound %q", scope, want)
	}
}
