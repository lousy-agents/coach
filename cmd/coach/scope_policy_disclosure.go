package main

import (
	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func withScopePolicy(report *codesignal.Report, project *codesignalcli.ProjectAnalysis) *codesignal.Report {
	if project == nil || project.PolicyScope.Path == "" {
		return report
	}
	out := *report
	policy := project.PolicyScope
	out.Scope.Policy = &policy
	return &out
}
