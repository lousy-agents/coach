package projectconfig

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// Whether the revision holds the policy in another case is a question about its tree. When git cannot
// answer it, or answers past its budget, the scan stops with the operational error that names the question:
// the policy is not called missing and the scan is not told its document is invalid, since nothing is known
// about either.
func TestDiscoverStopsWithAnOperationalErrorWhenGitCannotListTheRevisionsTree(t *testing.T) {
	repo := committedRepository(t)
	original := runProjectConfigGit
	t.Cleanup(func() { runProjectConfigGit = original })
	runProjectConfigGit = onlyTheListingFails(repo)

	selection, err := Discover(repo, "HEAD")

	var operational *gitrepo.OperationalError
	if !errors.As(err, &operational) || selection.Fatal != nil {
		t.Fatalf("Discover = %+v, %v; want the operational error for a listing git could not answer", selection, err)
	}
	if !errors.Is(operational.Cause, errBoom) {
		t.Errorf("the error's cause %v is not what git said", operational.Cause)
	}
	for _, want := range []string{"coach codesignal: ", CanonicalPath, listingQuestion} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error %q does not say %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "project_config_invalid") {
		t.Errorf("the error %q calls the policy invalid; git has not said what the revision holds", err)
	}
}

// onlyTheListingFails answers like a repository with a commit and no entry at any path, and fails the
// listing of a directory of the tree, which is the one question about the spellings the revision holds.
func onlyTheListingFails(repo string) func(string, ...string) ([]byte, error) {
	return func(_ string, args ...string) ([]byte, error) {
		switch {
		case args[0] == "ls-tree" && slices.Contains(args, "--name-only"):
			return nil, errBoom
		case args[0] == "rev-parse":
			return []byte(repo + "\n"), nil
		}
		return nil, nil
	}
}
