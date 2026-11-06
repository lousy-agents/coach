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
	named := "--project-config " + strconv.Quote(repoPath)
	reworded := *configErr
	reworded.Message = strings.Replace(configErr.Message, named, "the default policy "+strconv.Quote(repoPath), 1)
	return &reworded
}
