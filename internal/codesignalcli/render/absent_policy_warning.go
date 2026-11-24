package render

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// absentPolicyWarning replaces the generic differs-from-HEAD warning where the difference is that the policy HEAD
// selects is gone from disk, and says that the scan fell back to another file or to none.
func absentPolicyWarning(absent codesignal.Diagnostic, used *codesignal.PolicyScope) string {
	headPath := terminalText(absent.Path)
	if used == nil {
		return fmt.Sprintf("WARNING: %s is committed at HEAD%s and absent on disk%s; this scan used no project policy, so its result can differ from CI, which reads the committed policy.", headPath, anotherCaseClause(absent), ignoredFallbackClause(absent))
	}
	return fmt.Sprintf("WARNING: %s is committed at HEAD%s and absent on disk; this scan used %s instead, so its result can differ from CI, which reads the committed policy.", headPath, anotherCaseClause(absent), terminalText(used.Path))
}

func ignoredFallbackClause(absent codesignal.Diagnostic) string {
	if strings.Contains(absent.Message, codesignal.AbsentPolicyFallbackIgnored) {
		return ", and " + codesignal.AbsentPolicyFallbackIgnored
	}
	return ""
}

// anotherCaseClause names the default path a policy committed only in another case differs from, as the message carries
// it after codesignal.AbsentPolicyInAnotherCase.
func anotherCaseClause(absent codesignal.Diagnostic) string {
	_, after, found := strings.Cut(absent.Message, codesignal.AbsentPolicyInAnotherCase+" ")
	if !found {
		return ""
	}
	return " in another case than " + terminalText(strings.TrimSuffix(strings.Fields(after)[0], "."))
}
