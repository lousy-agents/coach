package main

import (
	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func withScopePolicy(report *codesignal.Report, dir string, project *codesignalcli.ProjectAnalysis) *codesignal.Report {
	if report == nil || project == nil || project.Policy.Path == "" {
		return report
	}
	out := *report
	out.Scope.Policy = &codesignal.PolicyScope{
		Path:        project.Policy.Path,
		Digest:      project.ConfigDigest,
		MatchesHead: policyOnDiskMatches(dir, project),
	}
	return &out
}

// A policy that cannot be selected from disk is reported as not matching rather than failing the scan.
func policyOnDiskMatches(dir string, project *codesignalcli.ProjectAnalysis) bool {
	namedPath := ""
	if project.Policy.Explicit {
		namedPath = project.Policy.Path
	}
	onDisk, err := projectconfig.SelectOnDisk(dir, namedPath)
	return err == nil && onDisk.Path == project.Policy.Path && onDisk.Digest == project.ConfigDigest
}
