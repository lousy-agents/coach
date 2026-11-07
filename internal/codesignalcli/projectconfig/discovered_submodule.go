package projectconfig

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// inTheSubmodule is selection with a regular file at the default policy's path set aside when the path lies inside
// a submodule. The superproject records a submodule as one commit, so such a file can be neither committed nor
// added here: to call it "not committed" tells the user to commit what git refuses to add. It cannot be the
// policy, and the scan says so as it does for the other entries that cannot, naming the submodule. A question git
// cannot answer is returned, never read as no submodule.
func inTheSubmodule(run gitrepo.Runner, dir string, selection Selection) (Selection, error) {
	if !selection.UncommittedCanonical {
		return selection, nil
	}
	submodule, err := enclosingSubmodule(run, dir, CanonicalPath)
	switch {
	case err != nil:
		return Selection{}, err
	case submodule == "":
		return selection, nil
	}
	return Selection{UnusableCanonical: &ConfigError{Kind: KindInvalid, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseInSubmodule, Blocker: submodule}}, nil
}
