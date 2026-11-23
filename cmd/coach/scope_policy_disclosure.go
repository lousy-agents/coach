package main

import (
	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func withScopePolicy(report *codesignal.Report, dir, revision string, project *codesignalcli.ProjectAnalysis) *codesignal.Report {
	if project == nil || project.Policy.Path == "" {
		return report
	}
	out := *report
	out.Scope.Policy = &codesignal.PolicyScope{
		Path:        project.Policy.Path,
		Digest:      project.ConfigDigest,
		MatchesHead: policyOnDiskMatches(dir, revision, project),
	}
	return &out
}

// A policy that cannot be selected from disk is reported as not matching rather than failing the scan. A file whose
// bytes differ still matches when git's conversion turns it back into the committed blob, because git status calls
// that checkout clean.
func policyOnDiskMatches(dir, revision string, project *codesignalcli.ProjectAnalysis) bool {
	namedPath := ""
	if project.Policy.Explicit {
		namedPath = project.Policy.Path
	}
	onDisk, err := projectconfig.SelectOnDisk(dir, namedPath)
	if err != nil || onDisk.Path != project.Policy.Path {
		return false
	}
	return onDisk.Digest == project.ConfigDigest || projectconfig.HoldsCommittedPolicyConverted(dir, revision, project.Policy.Path)
}
