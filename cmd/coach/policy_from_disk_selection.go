package main

import (
	"encoding/json"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// committedPolicy is the policy the analyzed revision holds. The zero value is no policy committed.
type committedPolicy struct {
	path    string
	content json.RawMessage
}

func (c committedPolicy) present() bool { return c.path != "" }

// diskPolicy is what the working tree selects. selection.Path is empty when nothing is selected, and refusal is set
// when a file is there but cannot be used.
type diskPolicy struct {
	selection projectconfig.DiskSelection
	refusal   *projectconfig.DiskPolicyRefusal
}

func (d diskPolicy) usable() bool { return d.refusal == nil && d.selection.Path != "" }

func (d diskPolicy) present() bool { return d.refusal != nil || d.selection.Path != "" }

// policyEvidence is everything the choice of policy depends on. The question about a path is a function so that the
// choice stays free of git.
type policyEvidence struct {
	committed        committedPolicy
	disk             diskPolicy
	holdsConvertedAt func(path string) bool
}

// policyChoice is the policy a scan uses. An empty scope.Path means the scan uses none.
type policyChoice struct {
	content     json.RawMessage
	scope       codesignal.PolicyScope
	diagnostics []codesignal.Diagnostic
}

// choosePolicy applies ADR-009 (#714): the policy on disk is the one used. The committed bytes stand in for it where they
// are the same file, so a clean or line-ending-converted checkout keeps the digest CI computes. A committed policy
// stands in for a policy on disk that is missing or unusable until the scan refuses those states. Discovery keeps its
// order: a committed canonical policy that is missing on disk is not replaced by the root project.json that is there.
func choosePolicy(e policyEvidence) policyChoice {
	if !e.disk.usable() || e.disk.selection.Path == projectconfig.DefaultPath && e.committed.path == projectconfig.CanonicalPath {
		return committedOrNone(e.committed, e.disk)
	}
	chosen := e.disk.selection
	if e.committed.path == chosen.Path {
		if projectconfig.Digest(e.committed.content) == chosen.Digest || e.holdsConvertedAt(chosen.Path) {
			return usedPolicy(e.committed.path, e.committed.content, codesignal.PolicyOriginMatchesHead, "")
		}
		return usedPolicy(chosen.Path, chosen.Content, codesignal.PolicyOriginDiffersFromHead, "")
	}
	return usedPolicy(chosen.Path, chosen.Content, codesignal.PolicyOriginNotCommitted, "")
}

func committedOrNone(committed committedPolicy, disk diskPolicy) policyChoice {
	switch {
	case committed.present() && disk.refusal != nil:
		return usedPolicy(committed.path, committed.content, codesignal.PolicyOriginDiskUnusable, disk.refusal.Reason)
	case committed.present():
		return usedPolicy(committed.path, committed.content, codesignal.PolicyOriginMissingOnDisk, "")
	case disk.refusal != nil:
		return policyChoice{diagnostics: []codesignal.Diagnostic{unusableOnDiskDiagnostic(disk.refusal)}}
	}
	return policyChoice{}
}

func usedPolicy(path string, content json.RawMessage, origin codesignal.PolicyOrigin, reason string) policyChoice {
	return policyChoice{
		content: content,
		scope: codesignal.PolicyScope{
			Path:        path,
			Digest:      projectconfig.Digest(content),
			MatchesHead: origin == codesignal.PolicyOriginMatchesHead,
			Origin:      origin,
			Reason:      reason,
		},
	}
}

func unusableOnDiskDiagnostic(refusal *projectconfig.DiskPolicyRefusal) codesignal.Diagnostic {
	return codesignal.Diagnostic{
		Kind:    "project_policy_unusable_on_disk",
		Path:    refusal.Path,
		Message: "the project policy on disk cannot be used (" + refusal.Reason + "); this scan used no policy",
	}
}
