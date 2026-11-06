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
	status, err := ExplicitOnDiskStatus(runProjectConfigGit, dir, revision, repoPath)
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
func ExplicitOnDiskStatus(run gitrepo.Runner, dir, revision, repoPath string) (*ConfigError, error) {
	err := requireOnDisk(run, dir, repoPath)
	if err == nil {
		return nil, nil
	}
	var found *ConfigError
	if !errors.As(err, &found) {
		return nil, err
	}
	judgment := judgeRestore(run, dir, revision, repoPath)
	judged := *found
	judged.FromWorkingTree = true
	judged.UnreadStagedEdit = judgment.indexEntryUnread
	if judgment.failure != nil {
		judged.GitFailure = judgment.failure
	}
	offer, offered := restoreOfferFor(judgment, revision, found, repoPath)
	if !offered {
		return &judged, nil
	}
	return annotated(&judged, offer), nil
}
