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

	scope := choosePolicy(policyEvidence{disk: disk, head: head, holdsConvertedAt: func(string) bool { return converted }}).scope

	wantDigest := disk.Digest
	if want == codesignal.PolicyCauseSameAsHead {
		wantDigest = projectconfig.Digest(head.Content)
	}
	if scope.Cause != want || scope.Path != disk.Path || scope.Digest != wantDigest || scope.MatchesHead != (want == codesignal.PolicyCauseSameAsHead) {
		t.Fatalf("scope = %+v, want cause %q at the path on disk with digest %q", scope, want, wantDigest)
	}
}

func TestChoosePolicyUsesTheCommittedPolicyWhereTheDiskHasNoneOrDiscoveryPrefersIt(t *testing.T) {
	committed := committedPolicy{path: projectconfig.CanonicalPath, content: []byte(`{"c":1}`)}
	legacyOnDisk := projectconfig.DiskSelection{Path: projectconfig.DefaultPath, Digest: "pcfg_x", Content: []byte(`{"d":1}`)}

	for name, disk := range map[string]projectconfig.DiskSelection{"no policy on disk": {}, "only the root project.json on disk": legacyOnDisk} {
		t.Run(name, func(t *testing.T) { expectCommittedAsMissing(t, committed, disk) })
	}

	t.Run("a committed root project.json does not outrank the canonical file on disk", func(t *testing.T) {
		canonicalOnDisk := projectconfig.DiskSelection{Path: projectconfig.CanonicalPath, Digest: "pcfg_x", Content: []byte(`{"d":1}`)}

		choice := choosePolicy(policyEvidence{committed: committedPolicy{path: projectconfig.DefaultPath}, disk: canonicalOnDisk, head: projectconfig.CommittedCopy{Kind: projectconfig.CopyAbsent}})

		if choice.scope.Path != projectconfig.CanonicalPath || choice.scope.Cause != codesignal.PolicyCauseNotCommitted {
			t.Fatalf("scope = %+v", choice.scope)
		}
	})
}

func expectCommittedAsMissing(t *testing.T, committed committedPolicy, disk projectconfig.DiskSelection) {
	t.Helper()
	choice := choosePolicy(policyEvidence{committed: committed, disk: disk})
	if choice.scope.Path != committed.path || choice.scope.Cause != codesignal.PolicyCauseMissingOnDisk {
		t.Fatalf("scope = %+v, want the committed policy as missing on disk", choice.scope)
	}
}

func TestChoosePolicyPrefersTheCommittedCanonicalOnlyOverTheRootProjectJSONOnDisk(t *testing.T) {
	absent := projectconfig.CommittedCopy{Kind: projectconfig.CopyAbsent}
	tests := []struct {
		name          string
		committedPath string
		diskPath      string
	}{
		{"the committed policy is the canonical one and so is the one on disk", projectconfig.CanonicalPath, projectconfig.CanonicalPath},
		{"the committed policy is the root project.json and so is the one on disk", projectconfig.DefaultPath, projectconfig.DefaultPath},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { expectDiskOverCommitted(t, tt.committedPath, tt.diskPath, absent) })
	}
}

func expectDiskOverCommitted(t *testing.T, committedPath, diskPath string, head projectconfig.CommittedCopy) {
	t.Helper()
	content := []byte(`{"d":1}`)
	disk := projectconfig.DiskSelection{Path: diskPath, Digest: projectconfig.Digest(content), Content: content}

	choice := choosePolicy(policyEvidence{committed: committedPolicy{path: committedPath, content: []byte(`{"c":1}`)}, disk: disk, head: head})

	if choice.scope.Cause != codesignal.PolicyCauseNotCommitted || choice.scope.Digest != disk.Digest {
		t.Fatalf("scope = %+v, want the policy on disk", choice.scope)
	}
}
