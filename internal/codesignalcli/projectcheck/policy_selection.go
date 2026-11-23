package projectcheck

import (
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func resolveCheckPolicy(dir, revision string, policy policyref.Ref) (string, []projectreadiness.Warning, error) {
	if policy.JudgedOnDisk() {
		return policy.Path, nil, nil
	}
	selection, err := projectconfig.Discover(dir, revision)
	if err != nil {
		return "", nil, err
	}
	return policyFromSelection(selection)
}

func policyFromSelection(selection projectconfig.Selection) (string, []projectreadiness.Warning, error) {
	switch {
	case selection.Fatal != nil && !canonicalRecheckedByReadiness(selection.Fatal):
		return "", nil, selection.Fatal
	case selection.Deprecated:
		return selection.Path, []projectreadiness.Warning{{Code: projectreadiness.WarnProjectConfigDeprecated}}, nil
	case selection.Path != "":
		return selection.Path, nil, nil
	default:
		return projectconfig.CanonicalPath, nil, nil
	}
}

// canonicalRecheckedByReadiness reports a canonical Load failure that
// checkPolicy must classify again. Load collapses a stdout size bound and a
// timeout or stderr bound into KindUnreadable. LoadForReadiness, which
// checkPolicy calls, reports the size bound as policy_invalid and leaves the
// timeout and stderr bounds as operational errors. Scan keeps Load's
// classification and does not call this path.
func canonicalRecheckedByReadiness(err error) bool {
	var configErr *projectconfig.ConfigError
	if !errors.As(err, &configErr) {
		return false
	}
	switch configErr.Kind {
	case projectconfig.KindInvalid, projectconfig.KindUnreadable:
		return true
	default:
		return false
	}
}

// PolicyRoots is the roots of the policy Run checks, read from the working tree as Run reads it, and none for a policy
// that is not usable.
func PolicyRoots(dir, revision string, policy policyref.Ref) ([]string, error) {
	judgment, err := judgePolicy(readinessGit(), dir, revision, policy)
	return judgment.roots, err
}
