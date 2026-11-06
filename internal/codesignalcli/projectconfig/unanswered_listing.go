package projectconfig

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// The question the revision's listing answers: whether the analyzed revision holds a policy's path in
// another case. A policy is not called missing, uncommitted or renamed while it is unanswered.
const listingQuestion = "whether the analyzed revision holds it in another case"

// unansweredListing is the refusal for a policy whose spellings in the analyzed revision git could not list.
// It names the question and git's words and offers nothing, since every remedy (author a policy, commit the
// file, move it) is wrong if the policy is there under another case. It carries the failure like every other
// question git could not answer, so readiness reports an operational error and not a verdict about the policy.
func unansweredListing(subject string, cause error) *ConfigError {
	failure := newGitFailure(listingQuestion, cause)
	return &ConfigError{Kind: KindUnreadable, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseGitUnreadable, Message: fmt.Sprintf("coach codesignal: %s (project_config_invalid): Coach could not check %s, so it gives no advice", subject, failure.describe()), GitFailure: failure}
}
