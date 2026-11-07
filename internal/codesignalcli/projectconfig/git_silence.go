package projectconfig

import (
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// GitSilence is the operational error for a refusal that is git not answering a question about the policy at
// policyPath, and nil for any other error. A question git left unanswered is an environment failure, not a
// verdict about the policy, so it ends the run (exit 1) in the words of the mode that asked, given as prefix.
// It returns an error and not the pointer, so that "git answered" is a nil error.
func GitSilence(refusal error, prefix, policyPath string) error {
	var config *ConfigError
	if !errors.As(refusal, &config) || config.GitFailure == nil {
		return nil
	}
	return &gitrepo.OperationalError{Message: fmt.Sprintf("%s: the policy at %q: %s", prefix, policyPath, config.GitFailure), Cause: errors.Unwrap(config.GitFailure)}
}
