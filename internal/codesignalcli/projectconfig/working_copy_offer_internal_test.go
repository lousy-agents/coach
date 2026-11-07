package projectconfig

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func TestNoReplacementIsOfferedForAFileWithWorkInIt(t *testing.T) {
	repo := committedInvalidPolicy(t)
	writeFile(t, repo, replacePath, validPolicy)
	invalid := invalidError(replacePath, "HEAD", "schema decode failed")

	settled := settledByTheWorkingCopy(runProjectConfigGit, repo, "HEAD", replacePath, invalid)

	var configErr *ConfigError
	if !asConfigError(settled, &configErr) || configErr.ReplacesPolicy() || configErr.PolicyCause != projectreadiness.CauseChangedOnDisk {
		t.Fatalf("settled = %v; want an error with no replacement and the cause changed_on_disk", settled)
	}
	for _, want := range []string{"differs from the committed one", "git diff HEAD -- ':(top,literal)cfg/p.json'", "commit it if it is the fix"} {
		if !strings.Contains(configErr.Message, want) {
			t.Errorf("message %q; want it to contain %q", configErr.Message, want)
		}
	}
}

func TestAStagedEditAndAConflictAreToldAsSuch(t *testing.T) {
	cases := map[string]struct {
		arrange func(t *testing.T, repo string)
		cause   projectreadiness.PolicyCause
		words   string
	}{
		"a staged edit": {func(t *testing.T, repo string) {
			writeFile(t, repo, replacePath, validPolicy)
			stage(t, repo, replacePath)
		}, projectreadiness.CauseStagedEdit, "git diff --cached -- ':(top,literal)cfg/p.json'"},
		"a conflict": {func(t *testing.T, repo string) { recordAConflict(t, repo, replacePath) }, projectreadiness.CauseMergeConflict, "resolve the conflict in it, mark it resolved, and commit it"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireTheWorkCause(t, c.arrange, c.cause, c.words) })
	}
}

func requireTheWorkCause(t *testing.T, arrange func(t *testing.T, repo string), cause projectreadiness.PolicyCause, words string) {
	t.Helper()
	repo := committedInvalidPolicy(t)
	arrange(t, repo)

	settled := settledByTheWorkingCopy(runProjectConfigGit, repo, "HEAD", replacePath, invalidError(replacePath, "HEAD", "bad"))

	var configErr *ConfigError
	if !asConfigError(settled, &configErr) || configErr.PolicyCause != cause || configErr.ReplacesPolicy() || !strings.Contains(configErr.Message, words) {
		t.Errorf("settled = %v; want the cause %q, no replacement, and %q", settled, cause, words)
	}
}

// A file that is the committed copy keeps the replacement, and so does an error that is not about a document a new one
// replaces, which is not even looked at.
func TestTheReplacementIsKeptForAFileThatHasNoWorkInIt(t *testing.T) {
	repo := committedInvalidPolicy(t)
	invalid := invalidError(replacePath, "HEAD", "schema decode failed")

	settled := settledByTheWorkingCopy(runProjectConfigGit, repo, "HEAD", replacePath, invalid)

	if settled != error(invalid) {
		t.Errorf("settled = %v; want the error as it was", settled)
	}
}

func TestAnErrorThatNoNewDocumentReplacesIsNotSettledAndGitIsNotAsked(t *testing.T) {
	asked := false
	gitThatNoOneShouldAsk := func(string, ...string) ([]byte, error) {
		asked = true
		return nil, errBoom
	}
	missing := &ConfigError{Kind: KindMissing, Missing: MissingAbsent, Remedy: projectreadiness.RemedyAuthor}

	for _, err := range []error{nil, missing, errBoom} {
		settled := settledByTheWorkingCopy(gitThatNoOneShouldAsk, t.TempDir(), "HEAD", replacePath, err)
		if settled != err || asked {
			t.Errorf("settled = %v for %v, asked git: %v; want it as it was and no question", settled, err, asked)
		}
	}
}

// Git that cannot say whether the file differs leaves the replacement out and says so: it is not read as a file with
// nothing in it.
func TestAFileGitCannotCompareGetsNoReplacement(t *testing.T) {
	gitThatAnswersNothing := func(string, ...string) ([]byte, error) { return nil, errBoom }

	settled := settledByTheWorkingCopy(gitThatAnswersNothing, t.TempDir(), "HEAD", replacePath, invalidError(replacePath, "HEAD", "bad"))

	var configErr *ConfigError
	if !asConfigError(settled, &configErr) || configErr.ReplacesPolicy() || configErr.PolicyCause != projectreadiness.CauseGitUnreadable {
		t.Errorf("settled = %v; want no replacement and the cause git_unreadable", settled)
	}
}
