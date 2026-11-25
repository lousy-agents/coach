package projectconfig

import (
	"errors"
	"strconv"
	"strings"
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
	reworded.Message = WordedAsTheDefaultPolicy(configErr.Message, repoPath)
	return &reworded
}

// WordedAsTheDefaultPolicy is message, written for a policy named with --project-config, worded for the policy Coach
// found: it names the default policy and not a flag the user never passed, because a user who follows a wording that
// names the flag switches the policy to the working tree's judgment and changes the verdict. The readiness check and the
// scan both word a discovered policy here, so the two cannot describe one file in two ways.
func WordedAsTheDefaultPolicy(message, repoPath string) string {
	named := "--project-config " + strconv.Quote(repoPath)
	return strings.Replace(message, named, "the default policy "+strconv.Quote(repoPath), 1)
}
