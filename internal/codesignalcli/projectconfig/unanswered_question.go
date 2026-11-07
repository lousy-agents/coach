package projectconfig

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// unansweredQuestion is unansweredListing for any question git left unanswered.
func unansweredQuestion(subject string, failure *gitFailure) *ConfigError {
	return &ConfigError{Kind: KindUnreadable, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseGitUnreadable, Message: fmt.Sprintf("coach codesignal: %s (project_config_invalid): Coach could not check %s, so it gives no advice", subject, failure.describe()), GitFailure: failure}
}
