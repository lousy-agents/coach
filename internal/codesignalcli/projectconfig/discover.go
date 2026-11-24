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
// uncommitted. Which spelling is read is the disk's to decide (WrongCaseOnDisk); a policy the revision commits only
// in another case is still the one it selects, so a scan that lacks it on disk says so and a check restores it.
func selectWhereNoBlobIsCommitted(run gitrepo.Runner, dir, root, revision string) (Selection, error) {
	refusal, err := NonFileAtRevision(run, dir, revision, CanonicalPath)
	if err != nil {
		return Selection{}, err
	}
	if refusal != nil {
		return Selection{Fatal: asDefaultPolicyError(refusal, CanonicalPath)}, nil
	}
	spelling, err := committedInAnotherCase(run, dir, revision)
	if err != nil {
		return Selection{}, err
	}
	if spelling != "" {
		return Selection{Path: spelling}, nil
	}
	return selectLegacy(run, dir, root, revision)
}

func loadCanonical(run gitrepo.Runner, dir, revision string) (Selection, error) {
	if _, err := loadThrough(run, dir, revision, CanonicalPath); err != nil {
		return Selection{Fatal: asDefaultPolicyError(err, CanonicalPath)}, nil
	}
	return Selection{Path: CanonicalPath}, nil
}
