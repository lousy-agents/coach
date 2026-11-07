package projectconfig

import (
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// leftToASubmodule is checked, with a difference in case below a submodule set aside. The superproject records a
// submodule as one commit, so its revision holds no spelling of anything inside, and a name there has no other
// spelling to be told apart from: to say that the revision holds neither would be false. The submodule decides,
// as it does for a name spelled as the checkout has it, so the mismatch is dropped and what it displaced, the
// missing file or nothing, is reported. A name that differs in case above the submodule, or in the submodule's own
// name, is not below one that the index records, and keeps the refusal. A question git cannot answer is returned
// as the error, which is never a verdict about the policy.
func leftToASubmodule(run gitrepo.Runner, dir, repoPath string, checked error) error {
	var mismatch caseMismatchError
	if !errors.As(checked, &mismatch) {
		return checked
	}
	submodule, err := enclosingSubmodule(run, dir, repoPath)
	switch {
	case err != nil:
		return err
	case submodule != "":
		return mismatch.displaced
	}
	return checked
}
