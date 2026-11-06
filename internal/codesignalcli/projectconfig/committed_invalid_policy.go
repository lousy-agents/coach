package projectconfig

import (
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// committedCopyProblem says why the policy committed at revision is not a valid policy, or "" when it is. A
// restore brings back exactly what is committed, so a copy the scan refuses is no answer to a policy that
// is gone from disk. A copy over the size budget is invalid for the reason a smaller policy fixes; a read
// that fails is an error, not a verdict, so a copy nobody has read is never called valid.
func committedCopyProblem(run gitrepo.Runner, dir, revision, repoPath string) (string, error) {
	data, err := run(dir, "show", revision+":"+repoPath)
	var bound *gitrepo.BoundError
	if errors.As(err, &bound) && bound.Kind == gitrepo.BoundStdout {
		return fmt.Sprintf("committed content exceeds the %d-byte size budget", MaxBytes), nil
	}
	if err != nil {
		return "", err
	}
	if invalid := validateJSON(data); invalid != nil {
		return invalid.Error(), nil
	}
	return "", nil
}

// judgeCommittedCopy turns a restore that would otherwise be offered into the verdict for a committed copy
// that is not valid, or into an undecidable one when the copy cannot be read, and reports false when the
// copy is valid, which leaves the restore as it was. A verdict that restores from the commit has no index
// entry that nothing has read, so none of these judgments claims one.
func judgeCommittedCopy(run gitrepo.Runner, dir, revision, repoPath string) (policyJudgment, bool) {
	problem, err := committedCopyProblem(run, dir, revision, repoPath)
	if err != nil {
		return policyJudgment{verdict: restoreUndecidable, failure: newGitFailure("what the committed copy holds", err)}, true
	}
	if problem == "" {
		return policyJudgment{}, false
	}
	return policyJudgment{verdict: restoreCommittedInvalid, committedInvalid: problem}, true
}
