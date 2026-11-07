package projectconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// A file at the default policy's path inside a submodule is set aside as the submodule's, named, and no longer
// counted as one that is not committed.
func TestAFileAtTheDefaultPathInsideASubmoduleIsSetAsideAsTheSubmodules(t *testing.T) {
	repo := repositoryWithASubmoduleAt(t, ".lousy-agents")

	settled, err := inTheSubmodule(runProjectConfigGit, repo, Selection{UncommittedCanonical: true})

	if err != nil || settled.UncommittedCanonical || settled.UnusableCanonical == nil {
		t.Fatalf("inTheSubmodule = %+v, %v; want the file set aside as unusable and not uncommitted", settled, err)
	}
	refusal := settled.UnusableCanonical
	if refusal.PolicyCause != projectreadiness.CauseInSubmodule || refusal.Blocker != ".lousy-agents" || refusal.Remedy != projectreadiness.RemedyNone {
		t.Errorf("refusal = %+v; want the cause in_submodule, the submodule named, and no way out", refusal)
	}
	if !settled.CanonicalOccupiedOnDisk() {
		t.Error("CanonicalOccupiedOnDisk = false; the path is taken, so project.json is not to be moved there")
	}
}

func TestTheMessageForAFileInsideASubmoduleNamesTheSubmoduleAndTheCause(t *testing.T) {
	refusal := &ConfigError{PolicyCause: projectreadiness.CauseInSubmodule, Blocker: ".lousy-agents"}

	message := UnusableCanonicalMessage(refusal)

	for _, want := range []string{"cause=in_submodule", `the submodule ".lousy-agents"`, "cannot be committed here"} {
		if !strings.Contains(message, want) {
			t.Errorf("message %q; want it to contain %q", message, want)
		}
	}
}

func TestAFileAtTheDefaultPathWhereNoSubmoduleIsRecordedStaysUncommitted(t *testing.T) {
	repo := repositoryWithASubmoduleAt(t, "vendor/sub")

	settled, err := inTheSubmodule(runProjectConfigGit, repo, Selection{UncommittedCanonical: true})

	if err != nil || !settled.UncommittedCanonical || settled.UnusableCanonical != nil {
		t.Errorf("inTheSubmodule = %+v, %v; want the file left as one that is not committed", settled, err)
	}
}

// Git that cannot say whether the path lies inside a submodule settles nothing: the failure is returned, and the
// file is neither called uncommitted nor the submodule's.
func TestAQuestionAboutASubmoduleThatGitCannotAnswerIsReturnedForTheDefaultPath(t *testing.T) {
	gitThatAnswersNothing := func(string, ...string) ([]byte, error) { return nil, errBoom }

	settled, err := inTheSubmodule(gitThatAnswersNothing, t.TempDir(), Selection{UncommittedCanonical: true})

	if !errors.Is(err, errBoom) || settled.UncommittedCanonical || settled.UnusableCanonical != nil {
		t.Errorf("inTheSubmodule = %+v, %v; want the failed question and no selection", settled, err)
	}
}

// What the disk does not hold is not the submodule's to settle, and git is not asked.
func TestWhereNothingIsOnDiskGitIsNotAskedAboutASubmodule(t *testing.T) {
	asked := false
	gitThatNoOneShouldAsk := func(string, ...string) ([]byte, error) {
		asked = true
		return nil, errBoom
	}
	unusable := Selection{UnusableCanonical: &ConfigError{PolicyCause: projectreadiness.CauseSymbolicLink}}

	for _, selection := range []Selection{{}, unusable} {
		settled, err := inTheSubmodule(gitThatNoOneShouldAsk, t.TempDir(), selection)
		if err != nil || asked || (settled.UnusableCanonical != selection.UnusableCanonical) {
			t.Errorf("inTheSubmodule(%+v) = %+v, %v, asked git: %v; want it passed on unchanged and no question", selection, settled, err, asked)
		}
	}
}
