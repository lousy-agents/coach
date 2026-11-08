package projectcheck

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

// onDiskStatus is what the working tree says about the policy path: nil for a regular file, the error that says
// why not otherwise, and the operational error when git could not answer a question the verdict depends on.
func onDiskStatus(run gitrepo.Runner, dir, revision, policyPath string) (*projectconfig.ConfigError, error) {
	status, err := projectconfig.ExplicitOnDiskStatus(run, dir, revision, policyPath)
	if err != nil {
		return nil, err
	}
	if status != nil && status.GitFailure != nil {
		return nil, unansweredQuestion(policyPath, status.GitFailure)
	}
	return status, nil
}
