package projectconfig

import (
	"errors"
	"testing"
)

var errWhatTheMismatchDisplaced = errors.New("the policy is missing from the directory the disk spells in another case")

// aMismatchBelow is a case mismatch at the component at index of repoPath, which the disk spells onDisk.
func aMismatchBelow(repoPath string, parts []string, index int, onDisk string, displaced error) caseMismatchError {
	mismatch := newCaseMismatch(repoPath, parts, index, onDisk)
	mismatch.displaced = displaced
	return mismatch
}

// The superproject records a submodule as one commit and holds no spelling of what is inside, so a difference in
// case below one is the submodule's to judge, and what the mismatch displaced is reported.
func TestADifferenceInCaseBelowASubmoduleIsSetAsideForWhatItDisplaced(t *testing.T) {
	repo := repositoryWithASubmoduleAt(t, "vendor/sub")
	const typed = "vendor/sub/CFG/p.json"
	checked := aMismatchBelow(typed, []string{"vendor", "sub", "CFG", "p.json"}, 2, "cfg", errWhatTheMismatchDisplaced)

	settled := leftToASubmodule(runProjectConfigGit, repo, typed, checked)

	if !errors.Is(settled, errWhatTheMismatchDisplaced) {
		t.Errorf("leftToASubmodule = %v; want what the mismatch displaced, %v", settled, errWhatTheMismatchDisplaced)
	}
}

func TestADifferenceInCaseBelowASubmoduleThatDisplacedNothingIsSetAsideForNothing(t *testing.T) {
	repo := repositoryWithASubmoduleAt(t, "vendor/sub")
	const typed = "vendor/sub/COACH.json"
	checked := aMismatchBelow(typed, []string{"vendor", "sub", "COACH.json"}, 2, "coach.json", nil)

	settled := leftToASubmodule(runProjectConfigGit, repo, typed, checked)

	if settled != nil {
		t.Errorf("leftToASubmodule = %v; want no refusal, since the submodule decides", settled)
	}
}

// A name that differs in case above the submodule, or in the submodule's own name, is not below one that the index
// records, and the revision can say which spelling it holds.
func TestADifferenceInCaseAboveASubmoduleKeepsItsRefusal(t *testing.T) {
	repo := repositoryWithASubmoduleAt(t, "vendor/sub")
	const typed = "Vendor/sub/cfg/p.json"
	checked := aMismatchBelow(typed, []string{"Vendor", "sub", "cfg", "p.json"}, 0, "vendor", errWhatTheMismatchDisplaced)

	settled := leftToASubmodule(runProjectConfigGit, repo, typed, checked)

	if !errors.As(settled, new(caseMismatchError)) {
		t.Errorf("leftToASubmodule = %v; want the case mismatch kept", settled)
	}
}

func TestADifferenceInCaseWhereNoSubmoduleIsRecordedKeepsItsRefusal(t *testing.T) {
	repo := repositoryWithASubmoduleAt(t, "vendor/sub")
	const typed = "docs/CFG/p.json"
	checked := aMismatchBelow(typed, []string{"docs", "CFG", "p.json"}, 1, "cfg", errWhatTheMismatchDisplaced)

	settled := leftToASubmodule(runProjectConfigGit, repo, typed, checked)

	if !errors.As(settled, new(caseMismatchError)) {
		t.Errorf("leftToASubmodule = %v; want the case mismatch kept", settled)
	}
}

// Git that cannot say whether the path lies inside a submodule settles nothing: the failure is returned, and
// neither the refusal nor what it displaced is taken for a verdict.
func TestAQuestionAboutASubmoduleThatGitCannotAnswerIsReturnedAndNotTakenForAVerdict(t *testing.T) {
	const typed = "vendor/sub/CFG/p.json"
	checked := aMismatchBelow(typed, []string{"vendor", "sub", "CFG", "p.json"}, 2, "cfg", errWhatTheMismatchDisplaced)
	gitThatAnswersNothing := func(string, ...string) ([]byte, error) { return nil, errBoom }

	settled := leftToASubmodule(gitThatAnswersNothing, t.TempDir(), typed, checked)

	if !errors.Is(settled, errBoom) {
		t.Errorf("leftToASubmodule = %v; want the failed question, %v", settled, errBoom)
	}
}

// What is not a case mismatch is not the submodule's to settle, and git is not asked.
func TestWhatIsNotACaseMismatchIsPassedOnWithoutAskingGit(t *testing.T) {
	asked := false
	gitThatNoOneShouldAsk := func(string, ...string) ([]byte, error) {
		asked = true
		return nil, errBoom
	}
	refusal := &ConfigError{Kind: KindMissing}

	settled := leftToASubmodule(gitThatNoOneShouldAsk, t.TempDir(), "vendor/sub/p.json", refusal)

	if settled != error(refusal) || asked {
		t.Errorf("leftToASubmodule = %v, asked git: %v; want the refusal as it was and no question", settled, asked)
	}
}
