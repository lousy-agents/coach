package projectconfig

import (
	"encoding/json"

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
// The disk is asked first whether it lists the path in another case, because the disk decides what the path names
// (WrongCaseOnDisk).
func ExplicitOnDiskStatus(run gitrepo.Runner, dir, revision, repoPath string) (*ConfigError, error) {
	return OnDiskStatus(run, dir, revision, repoPath, true)
}

// OnDiskStatus is ExplicitOnDiskStatus for a policy path the user named, or one Coach found where named is false.
func OnDiskStatus(run gitrepo.Runner, dir, revision, repoPath string, named bool) (*ConfigError, error) {
	if ValidatePath(repoPath) == nil {
		respelled, err := WrongCaseOnDisk(run, dir, repoPath, named)
		if err != nil {
			return nil, err
		}
		if respelled != nil {
			respelled.FromWorkingTree = true
			return respelled, nil
		}
	}
	return workingTreeStatus(run, dir, revision, repoPath)
}
