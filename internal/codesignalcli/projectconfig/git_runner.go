package projectconfig

import "github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"

// NewGitRunner is the git read behind every question Coach asks about a policy and the tree around it. Only
// the policy document is held to its own size limit (MaxBytes), so that one past it fails in under a second.
// Every other read has the budget of its subcommand, which is where gitrepo decides how much a listing of
// the revision's tree may hold: held to the document's limit, the listing of a repository root with a few
// thousand entries made the scan of a repository with no policy fail.
func NewGitRunner() gitrepo.Runner {
	document := gitrepo.BoundedRunner(MaxBytes, MaxGitStderr, GitTimeout)
	return func(dir string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "show" {
			return document(dir, args...)
		}
		return gitrepo.RunBytes(dir, args...)
	}
}
