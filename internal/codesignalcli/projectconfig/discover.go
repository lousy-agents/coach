package projectconfig

import (
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// Discover resolves the policy document committed at revision.
// Worktree presence is checked from the repository root, so a scan
// started in a subdirectory still sees CanonicalPath. Git failures
// are returned; they are not treated as a file-local scan.
func Discover(dir, revision string) (Selection, error) {
	root, err := gitrepo.RepositoryRoot(dir)
	if err != nil {
		return Selection{}, err
	}
	exists, err := gitrepo.FileExistsAtRevision(runProjectConfigGit, dir, revision, CanonicalPath)
	if err != nil {
		return Selection{}, err
	}
	if exists {
		return loadCanonical(dir, revision)
	}
	return selectLegacy(dir, root, revision)
}

func loadCanonical(dir, revision string) (Selection, error) {
	if _, err := Load(dir, revision, CanonicalPath); err != nil {
		return Selection{Fatal: err}, nil
	}
	return Selection{Path: CanonicalPath}, nil
}

func selectLegacy(dir, root, revision string) (Selection, error) {
	selection := Selection{UncommittedCanonical: configExistsInWorktree(root, CanonicalPath)}
	if _, err := Load(dir, revision, DefaultPath); err != nil {
		return legacyLoadFailure(selection, err)
	}
	selection.Path = DefaultPath
	selection.Deprecated = true
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
