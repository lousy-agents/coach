package gitrepo

import "fmt"

// ForeignGitDirectoryError is the refusal for a scan whose git directory belongs to a repository that does not
// hold the directory the scan runs in.
type ForeignGitDirectoryError struct {
	GitDir string
}

func (e *ForeignGitDirectoryError) Error() string {
	return fmt.Sprintf("the repository git reports (%s) is not the one that holds this working tree; if it is, rename a separate git directory that is named .git, or record the work tree with git config core.worktree <path>", TerminalSafe(e.GitDir))
}
