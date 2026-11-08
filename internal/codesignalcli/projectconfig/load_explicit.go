package projectconfig

import (
	"encoding/json"
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// LoadExplicit is Load for a --project-config the user named. It rejects a
// file that does not exist on disk even when the path is committed at the
// analyzed revision, because an explicit policy never falls back.
func LoadExplicit(dir, revision, repoPath string) (json.RawMessage, error) {
	status, err := ExplicitOnDiskStatus(NewGitRunner(), dir, revision, repoPath)
	if err != nil {
		return nil, err
	}
	if status != nil {
		return nil, status
	}
	return Load(dir, revision, repoPath)
}

// ExplicitOnDiskStatus is what the working tree says about a policy the user
// named with --project-config. It is nil when the path is a regular file under
// the repository root, and otherwise the *ConfigError that says why not and
// which way out, if any, is safe to offer. For every such error, missing or
// unusable, it also says whether the index holds an edit to the committed policy
// that nothing has read. The error return is an operational
// failure, never a verdict about the policy. run is the git runner that decides
// whether the policy is committed and what the index holds for it.
//
// The analyzed revision is asked first whether it commits the path in another case, because that decides what the path
// names whatever the disk shows: the answer is the same on a disk that folds case and on one that does not.
func ExplicitOnDiskStatus(run gitrepo.Runner, dir, revision, repoPath string) (*ConfigError, error) {
	if ValidatePath(repoPath) == nil {
		refusal := wrongCaseOfNamedPolicy(run, dir, revision, repoPath)
		if silence := GitSilence(refusal, "coach codesignal", repoPath); silence != nil {
			return nil, silence
		}
		var respelled *ConfigError
		if errors.As(refusal, &respelled) {
			respelled.FromWorkingTree = true
			return respelled, nil
		}
	}
	return workingTreeStatus(run, dir, revision, repoPath)
}
