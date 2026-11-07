package projectconfig

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// questionsOfTheInspection are the five questions inspectWorkingCopy and differsOnDisk ask git, each as the predicate
// that makes the whole fake git fail it. The file is compared with the committed copy by its bytes, so git is not asked
// for a diff, and the fake fails the test if it is.
var questionsOfTheInspection = []struct {
	name    string
	failing func([]string) bool
}{
	{"whether the revision has a commit", askingFor("cat-file", "^{commit}")},
	{"what the revision holds at the path", askingFor("ls-tree")},
	{"whether the committed blob is here", askingFor("cat-file", "0123456789012345678901234567890123456789")},
	{"what the index holds at the path", askingFor("ls-files")},
	{"where the repository's root is", askingFor("rev-parse")},
}

// Each question git cannot answer is a failure of the inspection and not a state of the file: no replacement is
// offered and no restore is, and the message says that Coach could not check. A failure read as an answer would print
// `mv -f` over a policy that may have been edited, or say that an index holds a staged edit that it was never asked about.
func TestEveryQuestionTheInspectionAsksIsAFailureWhenGitCannotAnswerIt(t *testing.T) {
	for _, question := range questionsOfTheInspection {
		assertAnUnansweredQuestionOffersNothing(t, question.name, question.failing)
	}
}

func assertAnUnansweredQuestionOffersNothing(t *testing.T, name string, failing func([]string) bool) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		root := aRootHoldingThePolicy(t)
		git := whollyFakeGit{t: t, root: root, failing: failing}

		settled := settledByTheWorkingCopy(git.run, root, "HEAD", replacePath, invalidError(replacePath, "HEAD", "schema decode failed"))

		message := requireNoReplacementAndNoRestore(t, settled)
		requireTheMessageSays(t, message, "could not check", "offers neither a restore nor a new policy")
		requireTheMessageSaysNone(t, message, "mv -f", "git restore", "staged edit")
	})
}

func requireNoReplacementAndNoRestore(t *testing.T, settled error) string {
	t.Helper()
	var configErr *ConfigError
	if !asConfigError(settled, &configErr) || configErr.ReplacesPolicy() || configErr.PolicyCause != projectreadiness.CauseGitUnreadable {
		t.Fatalf("settled = %v; want no replacement and the cause git_unreadable", settled)
	}
	return configErr.Message
}

func requireTheMessageSays(t *testing.T, message string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(message, want) {
			t.Errorf("message %q does not say %q", message, want)
		}
	}
}

func requireTheMessageSaysNone(t *testing.T, message string, unwanted ...string) {
	t.Helper()
	for _, word := range unwanted {
		if strings.Contains(message, word) {
			t.Errorf("message %q says %q: a replacement or a restore is printed, or the failure is read as a staged edit", message, word)
		}
	}
}
