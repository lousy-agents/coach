package projectconfig

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// namedForACommittedBlocker is the refusal to report for a path that runs through a file. The message tells the
// user to move or remove the file, which is right for a file nothing commits and wrong for one the analyzed
// revision commits: that file is the repository's, the path is the thing that is wrong, and advice to delete it
// asks the user to lose work. For a committed blocker the message says so and gives no such advice. The error
// return is a question git could not answer, never a verdict.
func namedForACommittedBlocker(run gitrepo.Runner, dir, revision, repoPath string, found *ConfigError) (*ConfigError, error) {
	if found.Blocker == "" {
		return found, nil
	}
	committed, err := gitrepo.FileExistsAtRevision(run, dir, revision, found.Blocker)
	if err != nil || !committed {
		return found, err
	}
	reworded := *found
	reworded.Message = fmt.Sprintf("coach codesignal: --project-config %q was not found on disk at the repository root (project_config_missing): a parent path component is not a directory, %q, which is a file the analyzed revision commits, so the path is probably wrong; the path is repository-relative", repoPath, found.Blocker)
	return &reworded, nil
}
