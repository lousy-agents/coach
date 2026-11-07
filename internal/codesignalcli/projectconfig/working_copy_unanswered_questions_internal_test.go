package projectconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// A required clean filter that fails makes `git diff` fail on a file whose stat information differs from the index,
// while the questions about the committed copy and the index entry are answered. The git is real: a comparison it
// could not make is not read as a file with nothing in it, which a new document would then replace.
func TestAFileWhoseCleanFilterFailsGetsNoReplacement(t *testing.T) {
	repo := committedInvalidPolicy(t)
	attributes := filepath.Join(repo, ".git", "info", "attributes")
	if err := os.MkdirAll(filepath.Dir(attributes), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(attributes, []byte(replacePath+" filter=broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitfixture.Git(t, repo, "config", "filter.broken.clean", "exit 1")
	gitfixture.Git(t, repo, "config", "filter.broken.required", "true")
	writeFile(t, repo, replacePath, validPolicy)

	settled := settledByTheWorkingCopy(runProjectConfigGit, repo, "HEAD", replacePath, invalidError(replacePath, "HEAD", "schema decode failed"))

	var configErr *ConfigError
	if !asConfigError(settled, &configErr) || configErr.ReplacesPolicy() || configErr.PolicyCause != projectreadiness.CauseGitUnreadable {
		t.Errorf("settled = %v; want no replacement and the cause git_unreadable", settled)
	}
}

// whollyFakeGit is the whole of git for inspectWorkingCopy: it answers each question the inspection asks, with the
// committed copy found and held by the index as it is, the repository's root where the file is, and a diff that says
// the file differs, and fails the one question named. Any other question is a failure of the test, so nothing is left to a real git.
type whollyFakeGit struct {
	t       *testing.T
	root    string
	failing string
}

func (g whollyFakeGit) run(_ string, args ...string) ([]byte, error) {
	g.t.Helper()
	if args[0] == g.failing {
		return nil, errors.New("exit status 128: fatal: unable to read the index")
	}
	switch args[0] {
	case "cat-file":
		return nil, nil
	case "ls-files":
		return []byte("H 100644 0123456789012345678901234567890123456789 0\t" + replacePath + "\x00"), nil
	case "ls-tree":
		return []byte("100644 blob 0123456789012345678901234567890123456789\t" + replacePath + "\x00"), nil
	case "rev-parse":
		return []byte(g.root + "\n"), nil
	case "diff":
		return []byte(replacePath + "\n"), nil
	}
	g.t.Errorf("a question the fake git does not know: git %s", strings.Join(args, " "))
	return nil, errors.New("unexpected question")
}

func aRootHoldingThePolicy(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(replacePath)), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, replacePath, validPolicy)
	return root
}

// The index is asked about before the file is compared, and a git that cannot list it has not said that the index
// holds the committed entry or nothing. The inspection is that failure, and not a file that differs.
func TestAnIndexThatCannotBeListedIsAFailureOfTheInspectionAndNotAFileThatDiffers(t *testing.T) {
	root := aRootHoldingThePolicy(t)
	git := whollyFakeGit{t: t, root: root, failing: "ls-files"}

	state, err := inspectWorkingCopy(git.run, root, "HEAD", replacePath)

	if err == nil || state != workingCopyUntouched {
		t.Errorf("inspectWorkingCopy = %v, %v; want the failure of the index question, and no state", state, err)
	}
}

// The control: with every question answered, the same fake says the file differs, so the failure above is the only
// thing that stops the inspection from saying so.
func TestTheFakeGitSaysTheFileDiffersWhenTheIndexCanBeListed(t *testing.T) {
	root := aRootHoldingThePolicy(t)
	git := whollyFakeGit{t: t, root: root}

	state, err := inspectWorkingCopy(git.run, root, "HEAD", replacePath)

	if err != nil || state != workingCopyChanged {
		t.Errorf("inspectWorkingCopy = %v, %v; want the file that differs", state, err)
	}
}
