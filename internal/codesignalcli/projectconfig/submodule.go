package projectconfig

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// UncommittedPolicy is what the scan says about a --project-config policy that exists in the
// working tree but is not committed at the analyzed revision, with the stable word for it. It is the
// one place that decides both, so the scan's message and the readiness result's cause cannot differ.
// A file inside a submodule cannot be committed here at all, which is a different way out from
// committing it, so it has a word of its own; and when git cannot say whether the path lies inside
// one, the message gives no advice, because "commit the file" is the circular advice for that case. A
// file the revision already holds in another case, on a repository that ignores case, is the same
// circle: git add records nothing for a rename that changes only case, so it has a word of its own.
// The error is that unanswered question, for a caller that reports a failure of git apart from a
// verdict about the policy: the scan words it in its message, and readiness returns it.
func UncommittedPolicy(run gitrepo.Runner, dir, revision, repoPath string) (string, projectreadiness.PolicyCause, error) {
	submodule, err := enclosingSubmodule(run, dir, repoPath)
	switch {
	case err != nil:
		return unansweredUncommitted(repoPath, revision, newGitFailure("whether it lies inside a submodule", err))
	case submodule != "":
		return fmt.Sprintf("coach codesignal: --project-config %q exists in the worktree inside the submodule %q, which this repository records as a single commit, so the file cannot be committed here (project_config_invalid): name a policy that is committed in this repository's own files", repoPath, submodule), projectreadiness.CauseInSubmodule, nil
	}
	var unanswered *gitFailure
	var renamed *ConfigError
	switch verdict := caseOnlyRenameRefusal(run, dir, revision, repoPath); {
	case errors.As(verdict, &unanswered):
		return unansweredUncommitted(repoPath, revision, unanswered)
	case errors.As(verdict, &renamed):
		return renamed.Message, projectreadiness.CauseNameCaseDiffers, nil
	}
	return fmt.Sprintf("coach codesignal: --project-config %q exists in the worktree but is not committed at revision %q (project_config_invalid): commit the file so it is readable at the analyzed revision", repoPath, revision), projectreadiness.CauseNotCommitted, nil
}

// unansweredUncommitted is what UncommittedPolicy says when git could not answer a question the advice
// depends on: the question, and no advice, because every way out of "not committed" is wrong for some answer.
func unansweredUncommitted(repoPath, revision string, failure *gitFailure) (string, projectreadiness.PolicyCause, error) {
	return fmt.Sprintf("coach codesignal: --project-config %q exists in the worktree but is not committed at revision %q (project_config_invalid): Coach could not check %s, so it gives no advice", repoPath, revision, failure.describe()), projectreadiness.CauseGitUnreadable, failure
}

// enclosingSubmodule is the submodule that is repoPath or whose checkout holds it, or "" when none does.
// The superproject records a submodule as one commit, so a file inside it can never be committed there,
// and telling the user to commit it sends them in a circle; and a path that is the submodule itself is
// not a file at all. An error means git could not say.
func enclosingSubmodule(run gitrepo.Runner, dir, repoPath string) (string, error) {
	parts := strings.Split(repoPath, "/")
	for i := 1; i <= len(parts); i++ {
		candidate := strings.Join(parts[:i], "/")
		recorded, err := recordedAsSubmodule(run, dir, candidate)
		if err != nil || recorded {
			return candidate, err
		}
	}
	return "", nil
}

// recordedAsSubmodule is whether the index holds a submodule (a 160000 entry) at exactly path, a path
// relative to the repository root. --full-name names each entry from the root as well, because git
// prints it relative to the directory it runs in otherwise. A submodule is one entry, so a listing that
// overruns the output budget is a directory with a great many files below it, and answers no: the
// entries below path cannot be left out of the question, because git reads a trailing slash on a
// submodule's path as the submodule itself.
func recordedAsSubmodule(run gitrepo.Runner, dir, path string) (bool, error) {
	output, err := run(dir, "ls-files", "--stage", "-z", "--full-name", "--", ":(top,literal)"+path)
	var bound *gitrepo.BoundError
	if errors.As(err, &bound) && bound.Kind == gitrepo.BoundStdout {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(gitrepo.SplitNULPaths(output), func(record string) bool {
		mode, name, found := strings.Cut(record, "\t")
		return found && gitrepo.SamePath(name, path) && strings.HasPrefix(mode, "160000 ")
	}), nil
}

// judgeInsideASubmodule is the judgment for a policy path inside a submodule, or for one that git could
// not say whether it is, and false for every other path. It comes before every other question, because
// nothing there can be restored or committed in this repository.
func judgeInsideASubmodule(run gitrepo.Runner, dir, repoPath string) (policyJudgment, bool) {
	submodule, err := enclosingSubmodule(run, dir, repoPath)
	if err != nil {
		return policyJudgment{verdict: restoreUndecidable, indexEntryUnread: true, failure: newGitFailure("whether it lies inside a submodule", err)}, true
	}
	return policyJudgment{verdict: restoreNotCommitted, submodule: submodule}, submodule != ""
}
