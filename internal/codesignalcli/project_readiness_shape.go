package codesignalcli

import (
	"path"
	"strings"
)

// checkProjectShape reports whether revision looks like a Node/TypeScript
// project at all: a committed package.json at the repository root, or, once
// policyPassed is true, at or above at least one of the policy's declared
// roots (nearest package.json walking toward the repository root, matching
// compiler resolution). Without a validated policy, a non-root package.json
// is not a reliable signal -- roots is untrusted input until a policy has
// passed schema/content validation -- so the root-only heuristic is the
// correct, conservative default when there is no other signal available.
// This is still a coarse, deliberately shallow signal: layer discovery
// beyond "does package.json exist here" is the policy check's job.
func checkProjectShape(dir, revision string, roots []string, policyPassed bool) (ReadinessCheck, error) {
	exists, err := fileExistsAtRevision(dir, revision, "package.json")
	if err != nil {
		return ReadinessCheck{}, err
	}
	if exists {
		return ReadinessCheck{State: ReadinessPass}, nil
	}

	if policyPassed {
		found, err := packageJSONExistsUnderAnyRoot(dir, revision, roots)
		if err != nil {
			return ReadinessCheck{}, err
		}
		if found {
			return ReadinessCheck{State: ReadinessPass}, nil
		}
	}

	return ReadinessCheck{State: ReadinessFail, Code: GapUnsupportedRepositoryShape}, nil
}

// packageJSONExistsUnderAnyRoot reports whether a package.json blob exists
// at or above any of roots at revision. The walk uses fileExistsAtRevision
// (Git snapshot), never worktree os.Stat, so it matches compiler
// nearest-manifest resolution without mixing host state into a snapshot
// check. roots: ["."] is skipped here because the caller already probed
// the repository-root package.json and must not walk down.
func packageJSONExistsUnderAnyRoot(dir, revision string, roots []string) (bool, error) {
	for _, root := range roots {
		found, err := packageJSONExistsWalkingUp(dir, revision, root)
		if err != nil || found {
			return found, err
		}
	}
	return false, nil
}

func packageJSONExistsWalkingUp(dir, revision, root string) (bool, error) {
	current := path.Clean(strings.TrimSpace(root))
	if current == "" || current == "." {
		return false, nil
	}
	for {
		exists, err := fileExistsAtRevision(dir, revision, path.Join(current, "package.json"))
		if err != nil || exists {
			return exists, err
		}
		parent := path.Dir(current)
		if parent == "." || parent == current {
			return false, nil
		}
		current = parent
	}
}
