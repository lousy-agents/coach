package projectconfig

import (
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

func selectLegacy(run gitrepo.Runner, dir, root, revision string) (Selection, error) {
	selection, err := canonicalOnDisk(root)
	if err != nil {
		return Selection{}, err
	}
	selection, err = inTheSubmodule(run, dir, selection)
	if err != nil {
		return Selection{}, err
	}
	if _, err := loadThrough(run, dir, revision, DefaultPath); err != nil {
		return legacyLoadFailure(selection, asDefaultPolicyError(err, DefaultPath))
	}
	selection.Path = DefaultPath
	selection.Deprecated = true
	return heldByTheIndex(run, dir, revision, selection)
}

// heldByTheIndex is selection with the default policy's path set aside when nothing is on disk there and the index
// or a submodule still holds it, which the judgment of the path decides as it does for a policy named with
// --project-config. It asks only where project.json is the policy and the disk is free, which is where a move to the
// path could be offered. A question git cannot answer is returned, never read as a free path.
func heldByTheIndex(run gitrepo.Runner, dir, revision string, selection Selection) (Selection, error) {
	if !selection.Deprecated || selection.CanonicalPathTaken() {
		return selection, nil
	}
	status, err := workingTreeStatus(run, dir, revision, CanonicalPath)
	if err != nil {
		return Selection{}, err
	}
	if status != nil && status.AbsentFromDisk() && !status.FreeForANewPolicy() {
		selection.HeldCanonical = status
	}
	return selection, nil
}

func legacyLoadFailure(selection Selection, err error) (Selection, error) {
	var configErr *ConfigError
	if !errors.As(err, &configErr) {
		return Selection{}, err
	}
	switch configErr.Kind {
	case KindNotFound, KindInvalid, KindUncommitted:
		return selection, nil
	default:
		return Selection{}, err
	}
}
