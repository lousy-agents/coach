package render

import (
	"fmt"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// absentPolicyWarning replaces the generic differs-from-HEAD warning where the difference is that the policy HEAD
// selects is gone from disk, and says that the scan fell back to another file or to none.
func absentPolicyWarning(headPath string, used *codesignal.PolicyScope) string {
	if used == nil {
		return fmt.Sprintf("WARNING: %s is committed at HEAD and absent on disk; this scan used no project policy, so its result can differ from CI, which reads the committed policy.", terminalText(headPath))
	}
	return fmt.Sprintf("WARNING: %s is committed at HEAD and absent on disk; this scan used %s instead, so its result can differ from CI, which reads the committed policy.", terminalText(headPath), terminalText(used.Path))
}
