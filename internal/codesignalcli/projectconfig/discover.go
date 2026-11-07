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
	root, err := gitrepo.RepositoryRootWith(runProjectConfigGit, dir)
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
	return selectWhereNoBlobIsCommitted(dir, root, revision)
}

// selectWhereNoBlobIsCommitted is discovery for a canonical path that holds no blob at revision. A
// directory or a submodule committed there is still what the revision holds at the policy's path, so
// the scan refuses it as readiness does and does not fall back to a legacy policy or call the path
// uncommitted. A policy the revision holds at the same path in another case is refused too, with the
// commands that move it, since falling back would run the scan without a policy that is committed.
// Only a path with nothing like it falls back.
func selectWhereNoBlobIsCommitted(dir, root, revision string) (Selection, error) {
	refusal, err := NonFileAtRevision(runProjectConfigGit, dir, revision, CanonicalPath)
	if err != nil {
		return Selection{}, err
	}
	if refusal != nil {
		return Selection{Fatal: asDefaultPolicyError(refusal, CanonicalPath)}, nil
	}
	if renamed := WrongCaseAtRevision(runProjectConfigGit, dir, revision, CanonicalPath); renamed != nil {
		if silence := GitSilence(renamed, "coach codesignal", CanonicalPath); silence != nil {
			return Selection{}, silence
		}
		return Selection{Fatal: renamed}, nil
	}
	return selectLegacy(dir, root, revision)
}

func loadCanonical(dir, revision string) (Selection, error) {
	if _, err := Load(dir, revision, CanonicalPath); err != nil {
		return Selection{Fatal: asDefaultPolicyError(err, CanonicalPath)}, nil
	}
	return Selection{Path: CanonicalPath}, nil
}

func selectLegacy(dir, root, revision string) (Selection, error) {
	selection, err := canonicalOnDisk(root)
	if err != nil {
		return Selection{}, err
	}
	if _, err := Load(dir, revision, DefaultPath); err != nil {
		return legacyLoadFailure(selection, asDefaultPolicyError(err, DefaultPath))
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
