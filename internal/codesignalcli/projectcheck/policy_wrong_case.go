package projectcheck

import (
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// asWrongCase is the refusal of a policy the disk lists only in another case, from a selection's error, or nil.
func asWrongCase(err error) *projectconfig.ConfigError {
	var refusal *projectconfig.ConfigError
	if errors.As(err, &refusal) && refusal.PolicyCause == projectreadiness.CauseNameCaseDiffers {
		return refusal
	}
	return nil
}

// wrongCasePath is the path a case refusal is about: the one named, or the canonical path for a policy Coach finds.
func wrongCasePath(namedPath string) string {
	if namedPath != "" {
		return namedPath
	}
	return projectconfig.CanonicalPath
}

func judgmentOfWrongCase(renamed error, path string, policy policyref.Ref) (policyJudgment, error) {
	check, _, err := policyGapFrom(renamed)
	if err != nil {
		return policyJudgment{}, err
	}
	return policyJudgment{path: path, check: wordedForThePolicy(check, policy, path)}, nil
}
