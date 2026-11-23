package projectconfig

import (
	"fmt"

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

// SubmoduleRefusal is the error a scan stops with when the default policy's path lies inside a submodule checkout, or
// nil when it does not. Whatever the checkout holds there cannot be committed here, so it is not read as a policy.
func (s Selection) SubmoduleRefusal() error {
	refusal := s.UnusableCanonical
	if refusal == nil || refusal.PolicyCause != projectreadiness.CauseInSubmodule {
		return nil
	}
	return &ConfigError{
		Kind:        KindInvalid,
		Remedy:      projectreadiness.RemedyNone,
		PolicyCause: refusal.PolicyCause,
		Blocker:     refusal.Blocker,
		Message:     fmt.Sprintf("coach codesignal: %q cannot be used on disk (project_config_invalid): it lies inside the submodule %q, which this repository records as a single commit, so the file cannot be committed here (cause=%s)", CanonicalPath, refusal.Blocker, refusal.PolicyCause),
	}
}
