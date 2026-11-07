package gitrepo

import (
	"fmt"
	"os/exec"
	"strings"
)

// ResolveRevisions verifies dir is a Git worktree, resolves HEAD and base
// to full commit SHAs, and returns HEAD's SHA plus the merge-base of base
// and HEAD. Any failure is returned as an *OperationalError.
func ResolveRevisions(dir, base string) (headSHA, mergeBaseSHA string, err error) {
	return resolveRevisionsBy(RunBytes, dir, base)
}

// resolveRevisionsBy is ResolveRevisions with the git that answers given by the caller, so that a test can stall one
// of the reads and see that each reports its own timeout.
func resolveRevisionsBy(run Runner, dir, base string) (headSHA, mergeBaseSHA string, err error) {
	headSHA, err = resolveHEAD(run, dir)
	if err != nil {
		return "", "", err
	}

	if _, runErr := run(dir, "rev-parse", "--verify", base+"^{commit}"); runErr != nil {
		if bound := BoundFailure(runErr); bound != nil {
			return "", "", bound
		}
		return "", "", &OperationalError{Message: fmt.Sprintf("coach codesignal: --base %q cannot be resolved to a commit", base)}
	}

	mergeBaseOutput, runErr := run(dir, "merge-base", base, "HEAD")
	if runErr != nil {
		if bound := BoundFailure(runErr); bound != nil {
			return "", "", bound
		}
		return "", "", &OperationalError{Message: fmt.Sprintf("coach codesignal: no merge base between %q and HEAD", base)}
	}
	mergeBaseSHA = strings.TrimSpace(string(mergeBaseOutput))

	return headSHA, mergeBaseSHA, nil
}

// ResolveBaselineRevision verifies dir is a Git worktree and resolves HEAD
// to a full commit SHA for a Repository Baseline run, which has no base or
// merge-base to resolve. Any failure is returned as an *OperationalError,
// with the same messages ResolveRevisions uses for the checks they share.
func ResolveBaselineRevision(dir string) (revisionSHA string, err error) {
	return resolveHEAD(RunBytes, dir)
}

// resolveHEAD verifies dir is a Git worktree and resolves HEAD to a full
// commit SHA. It backs both ResolveRevisions and ResolveBaselineRevision so
// the two share identical operational-error messages for the checks they
// have in common.
func resolveHEAD(run Runner, dir string) (string, error) {
	if _, lookErr := exec.LookPath("git"); lookErr != nil {
		return "", gitNotFoundError()
	}

	worktreeOutput, runErr := run(dir, "rev-parse", "--is-inside-work-tree")
	if bound := BoundFailure(runErr); bound != nil {
		return "", bound
	}
	if runErr != nil || strings.TrimSpace(string(worktreeOutput)) != "true" {
		return "", &OperationalError{
			Message: fmt.Sprintf("coach codesignal: %s is not inside a Git worktree", dir),
			reason:  "not inside a Git worktree",
		}
	}

	headOutput, runErr := run(dir, "rev-parse", "HEAD")
	if runErr != nil {
		if bound := BoundFailure(runErr); bound != nil {
			return "", bound
		}
		return "", &OperationalError{Message: "coach codesignal: HEAD is not readable (does the repository have any commits?)"}
	}
	return strings.TrimSpace(string(headOutput)), nil
}

// gitNotFoundError is the operational error for a git that is not installed, worded once for every read.
func gitNotFoundError() *OperationalError {
	return &OperationalError{Message: "coach codesignal: git executable not found in PATH"}
}
