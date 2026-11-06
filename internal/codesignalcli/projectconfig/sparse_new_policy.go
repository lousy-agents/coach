package projectconfig

import (
	"path"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// sparseNewPolicyOffer keeps the offer to write a new policy and says how to stage it, for a policy
// that nothing commits in a repository with a sparse checkout whose directory is not on disk. A
// directory the sparse checkout leaves out is not on disk either, and git add refuses a path outside
// the checkout, so the new file would be written and could not be staged as it is.
var sparseNewPolicyOffer = restoreOffer{
	remedy: projectreadiness.RemedyAuthor,
	cause:  "",
	text:   ". If its directory is outside the sparse checkout, add the directory with git sparse-checkout add, or stage the new policy with git add --sparse",
}

// newPolicyMayNeedSparse is whether a new policy at repoPath could be written outside a sparse
// checkout: core.sparseCheckout is on and the policy's directory is not on disk (the repository root,
// which a policy at the top level names as ".", always is). It cannot tell a
// directory the checkout leaves out from one that was never there, so the note it leads to is
// conditional. Where git cannot say, the answer is no: the note is advice added to an offer, and
// a question it cannot answer is not a reason to withhold the offer.
func newPolicyMayNeedSparse(run gitrepo.Runner, dir, repoPath string) bool {
	parent := path.Dir(repoPath)
	root, err := gitrepo.RepositoryRootWith(run, dir)
	if err != nil || worktreeHolds(root, parent) {
		return false
	}
	out, err := run(dir, "config", "--type=bool", "--get", "core.sparseCheckout")
	return err == nil && strings.TrimSpace(string(out)) == "true"
}
