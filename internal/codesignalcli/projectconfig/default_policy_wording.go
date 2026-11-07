package projectconfig

import (
	"errors"
	"strconv"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// asDefaultPolicyError is err worded for a policy the scan found by itself: when it is a *ConfigError, its
// message names the default policy, where it would name the flag --project-config, which the user never
// passed. Any other error is returned as it is.
func asDefaultPolicyError(err error, repoPath string) error {
	var configErr *ConfigError
	if !errors.As(err, &configErr) {
		return err
	}
	reworded := *configErr
	reworded.Message = WordedAsTheDefaultPolicy(configErr.Message, configErr.PolicyCause, repoPath)
	return &reworded
}

// WordedAsTheDefaultPolicy is message, written for a policy named with --project-config, worded for the policy Coach
// found: it names the default policy and not a flag the user never passed, because a user who follows a wording that
// names the flag switches the policy to the working tree's judgment and changes the verdict. A file that is only not
// committed is no refusal where Coach found the policy, since a scan goes on and says so in a project_config_uncommitted
// diagnostic, so the reason carries that word and not the code of a refusal. The readiness check and the scan both
// word a discovered policy here, so the two cannot describe one file in two ways.
func WordedAsTheDefaultPolicy(message string, cause projectreadiness.PolicyCause, repoPath string) string {
	named := "--project-config " + strconv.Quote(repoPath)
	reworded := strings.Replace(message, named, "the default policy "+strconv.Quote(repoPath), 1)
	if cause == projectreadiness.CauseNotCommitted {
		reworded = strings.Replace(reworded, "(project_config_invalid)", "(project_config_uncommitted)", 1)
	}
	return reworded
}
