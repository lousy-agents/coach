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

// A required clean filter that fails would have made `git diff` fail on a file whose stat information differs from the
// index. The file is compared by its bytes with the committed copy, so the filter is not run and cannot be what decides:
// a file with a fix in it has work in it, and one that is the committed copy has none. The git is real.
func TestAFileWhoseCleanFilterFailsIsComparedByItsBytesAndTheFilterIsNotRun(t *testing.T) {
	cases := map[string]struct {
		content string
		replace bool
		cause   projectreadiness.PolicyCause
	}{
		"a fix in the file":           {validPolicy, false, projectreadiness.CauseChangedOnDisk},
		"the committed copy as it is": {`{"bogus":1}`, true, ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireSettledWithAFailingFilter(t, c.content, c.replace, c.cause) })
	}
}

func requireSettledWithAFailingFilter(t *testing.T, content string, replace bool, cause projectreadiness.PolicyCause) {
	t.Helper()
	repo := committedInvalidPolicy(t)
	withARequiredCleanFilterThatFails(t, repo)
	writeFile(t, repo, replacePath, content)

	settled := settledByTheWorkingCopy(NewGitRunner(), repo, "HEAD", replacePath, invalidError(replacePath, "HEAD", "schema decode failed"))

	var configErr *ConfigError
	if !asConfigError(settled, &configErr) || configErr.ReplacesPolicy() != replace || configErr.PolicyCause != cause {
		t.Errorf("settled = %v; want replacement offered = %v and the cause %q", settled, replace, cause)
	}
}

func withARequiredCleanFilterThatFails(t *testing.T, repo string) {
	t.Helper()
	attributes := filepath.Join(repo, ".git", "info", "attributes")
	if err := os.MkdirAll(filepath.Dir(attributes), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(attributes, []byte(replacePath+" filter=broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitfixture.Git(t, repo, "config", "filter.broken.clean", "exit 1")
	gitfixture.Git(t, repo, "config", "filter.broken.required", "true")
}

// askingFor is a question named by the git subcommand and, when the subcommand is asked more than once, by a word in
// its arguments.
func askingFor(subcommand string, containing ...string) func([]string) bool {
	return func(args []string) bool {
		return args[0] == subcommand && (len(containing) == 0 || strings.Contains(strings.Join(args, " "), containing[0]))
	}
}

// whollyFakeGit is the whole of git for inspectWorkingCopy: it answers each question the inspection asks, with the
// committed copy found and held by the index as it is, and the repository's root where the file is, and fails the one
// question named. The file differs from the committed copy because neither its bytes nor its conversion hash to the
// blob it names. Any other question, a diff above all, is a failure of the test, so nothing is left to a real git.
type whollyFakeGit struct {
	t       *testing.T
	root    string
	failing func(args []string) bool
}

func (g whollyFakeGit) run(_ string, args ...string) ([]byte, error) {
	g.t.Helper()
	if g.failing != nil && g.failing(args) {
		return nil, errors.New("exit status 128: fatal: git could not answer")
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
	case "check-attr":
		return []byte(replacePath + "\x00filter\x00unspecified\x00"), nil
	case "hash-object":
		return []byte(strings.Repeat("9", 40) + "\n"), nil
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
	git := whollyFakeGit{t: t, root: root, failing: askingFor("ls-files")}

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
