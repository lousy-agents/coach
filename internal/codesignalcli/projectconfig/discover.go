package projectconfig

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// Discover resolves the policy document committed at revision.
// Worktree presence is checked from the repository root, so a scan
// started in a subdirectory still sees CanonicalPath. Git failures
// are returned; they are not treated as a file-local scan.
func Discover(dir, revision string) (Selection, error) {
	return discoverThrough(NewGitRunner(), dir, revision)
}

// discoverThrough is Discover with every git question asked of run.
func discoverThrough(run gitrepo.Runner, dir, revision string) (Selection, error) {
	root, err := gitrepo.RepositoryRootWith(run, dir)
	if err != nil {
		return Selection{}, err
	}
	exists, err := gitrepo.FileExistsAtRevision(run, dir, revision, CanonicalPath)
	if err != nil {
		return Selection{}, err
	}
	if exists {
		return loadCanonical(run, dir, revision)
	}
	return selectWhereNoBlobIsCommitted(run, dir, root, revision)
}

// selectWhereNoBlobIsCommitted is discovery for a canonical path that holds no blob at revision. A
// directory or a submodule committed there is still what the revision holds at the policy's path, so
// the scan refuses it as readiness does and does not fall back to a legacy policy or call the path
// uncommitted. A policy the revision holds at the same path in another case is refused too, with the
// commands that move it, since falling back would run the scan without a policy that is committed.
// Only a path with nothing like it falls back.
func selectWhereNoBlobIsCommitted(run gitrepo.Runner, dir, root, revision string) (Selection, error) {
	refusal, err := NonFileAtRevision(run, dir, revision, CanonicalPath)
	if err != nil {
		return Selection{}, err
	}
	if refusal != nil {
		return Selection{Fatal: asDefaultPolicyError(refusal, CanonicalPath)}, nil
	}
	if renamed := WrongCaseAtRevision(run, dir, revision, CanonicalPath); renamed != nil {
		if silence := GitSilence(renamed, "coach codesignal", CanonicalPath); silence != nil {
			return Selection{}, silence
		}
		return Selection{Fatal: renamed}, nil
	}
	return selectLegacy(run, dir, root, revision)
}

func loadCanonical(run gitrepo.Runner, dir, revision string) (Selection, error) {
	if _, err := loadThrough(run, dir, revision, CanonicalPath); err != nil {
		return Selection{Fatal: asDefaultPolicyError(err, CanonicalPath)}, nil
	}
	return Selection{Path: CanonicalPath}, nil
}
