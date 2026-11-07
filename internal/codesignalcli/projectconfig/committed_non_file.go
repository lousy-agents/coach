package projectconfig

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// notAFileRefusal is how a policy that a revision commits as something other than a file is
// named: the stable word, and the sentence that says what it is and what to do instead. None of
// them offers a new policy or a restore, because the committed entry stays where it is.
type notAFileRefusal struct {
	cause  projectreadiness.PolicyCause
	reason string
}

var notAFileRefusals = map[gitrepo.EntryKind]notAFileRefusal{
	gitrepo.EntrySymbolicLink: {projectreadiness.CauseCommittedAsLink, "it is committed as a symbolic link, which a policy cannot be, so there is nothing to restore; name another policy or commit a regular file at this path"},
	gitrepo.EntryDirectory:    {projectreadiness.CauseDirectory, "it is committed as a directory, which a policy cannot be; name a file"},
	gitrepo.EntrySubmodule:    {projectreadiness.CauseInSubmodule, "it is committed as a submodule, which a policy cannot be; name a policy this repository commits"},
	gitrepo.EntryOther:        {projectreadiness.CauseNotARegularFile, "it is committed as an entry that is not a regular file, which a policy cannot be; commit a regular file at this path"},
}

// NonFileAtRevision is the *ConfigError for a policy that revision commits as a link, a directory, a
// submodule or any other entry that is not a regular file, and nil when revision holds a regular
// file at the path or nothing. It reads the entry's mode and type before any content, because a
// link's content is a path and a directory's is a listing: reading either as a document blames
// the author for a file that is not there. The error return is a failed question, never a verdict.
func NonFileAtRevision(run gitrepo.Runner, dir, revision, repoPath string) (*ConfigError, error) {
	kind, err := gitrepo.EntryKindAtRevision(run, dir, revision, repoPath)
	if err != nil {
		return nil, err
	}
	refusal, notAFile := notAFileRefusals[kind]
	if !notAFile {
		return nil, nil
	}
	return &ConfigError{
		Kind:        KindInvalid,
		Remedy:      projectreadiness.RemedyNone,
		PolicyCause: refusal.cause,
		Message:     fmt.Sprintf("coach codesignal: --project-config %q is invalid at revision %q (project_config_invalid): %s", repoPath, revision, refusal.reason),
	}, nil
}

// refuseBeforeReading is the error a read of repoPath at revision stops at before any content is
// read: a path that cannot name a policy, or an entry that is not a regular file. A question git
// cannot answer is not guessed at; unanswered words it for the caller.
func refuseBeforeReading(dir, revision, repoPath string, unanswered func(error) error) error {
	if err := ValidatePath(repoPath); err != nil {
		return invalidPathError(repoPath, revision, err.Error())
	}
	refusal, err := NonFileAtRevision(runProjectConfigGit, dir, revision, repoPath)
	if err != nil {
		return unanswered(err)
	}
	if refusal != nil {
		return refusal
	}
	return nil
}

// refuseScanRead is refuseBeforeReading for a scan, which says it could not check and offers nothing.
func refuseScanRead(dir, revision, repoPath string) error {
	return refuseBeforeReading(dir, revision, repoPath, func(err error) error {
		return unansweredRead(repoPath, revision, newGitFailure("what the revision holds at the path", err))
	})
}

// refuseReadinessRead is refuseBeforeReading for readiness, which fails closed with git's own error.
func refuseReadinessRead(dir, revision, repoPath string) error {
	return refuseBeforeReading(dir, revision, repoPath, func(err error) error { return err })
}
