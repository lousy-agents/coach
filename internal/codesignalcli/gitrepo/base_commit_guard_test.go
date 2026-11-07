package gitrepo

import (
	"slices"
	"strings"
	"testing"
)

// On a git that cannot be told not to fetch, the merge base is found by reading commits, and a commit that is not
// here is fetched. So the commit --base names is asked about first, with questions that read no object, and
// the merge base is asked for only once it is here.
func TestOnAnOldGitABaseCommitThatIsNotHereIsRefusedBeforeTheMergeBaseIsAsked(t *testing.T) {
	repository := fakeRepository{version: oldGit, promisors: promisorOrigin, mergeBase: mergeBaseSHA, probes: map[string]probeAnswer{firstMissing: {output: missingIDs(firstMissing)}}}

	err := RefuseMissingObjects(repository.run, "repo", "refs/pull/7/head")

	if len(repository.mergeBaseAsked) != 0 {
		t.Errorf("merge-base was asked %v; a commit that is not here is fetched by it", repository.mergeBaseAsked)
	}
	for _, word := range []string{"git 2.43.0 cannot be told not to fetch missing objects", "the commit --base names has 1 object that is not in it", "use git 2.44 or later", firstMissing} {
		if err == nil || !strings.Contains(err.Error(), word) {
			t.Errorf("RefuseMissingObjects = %v; want it to say %q", err, word)
		}
	}
	if slices.Contains(repository.probed, "rev-list --objects --missing=print --no-walk --quiet HEAD") {
		t.Errorf("probed %q; the refusal comes before anything else is read", repository.probed)
	}
}

func TestOnAnOldGitABaseCommitThatIsHereGoesOnToTheMergeBase(t *testing.T) {
	repository := fakeRepository{version: oldGit, mergeBase: mergeBaseSHA}

	err := RefuseMissingObjects(repository.run, "repo", "origin/main")

	if err != nil || len(repository.mergeBaseAsked) != 1 {
		t.Errorf("RefuseMissingObjects = %v after merge-base %v; want no refusal and the merge base asked once", err, repository.mergeBaseAsked)
	}
}

func TestOnAnOldGitABaseThatDoesNotResolveIsLeftToTheScanWithoutAskingForItsCommit(t *testing.T) {
	repository := fakeRepository{version: oldGit, revParseErr: exitStatusOne}

	err := RefuseMissingObjects(repository.run, "repo", "no-such-ref")

	if err != nil || slices.ContainsFunc(repository.probed, func(probe string) bool { return !strings.HasSuffix(probe, " HEAD") }) {
		t.Errorf("RefuseMissingObjects = %v after probing %q; want nil, and no probe of a commit the base does not name", err, repository.probed)
	}
}

// A name that begins with a dash is read as an option by a git too old for --end-of-options, so it is not
// asked; the merge base's own question has a "--" before it, and the scan words what that says.
func TestOnAnOldGitABaseThatLooksLikeAnOptionIsNotHandedToRevParse(t *testing.T) {
	asked := false
	repository := fakeRepository{version: oldGit}
	run := func(dir string, args ...string) ([]byte, error) {
		asked = asked || (args[0] == "rev-parse" && slices.Contains(args, "--upload-pack=touch"))
		return repository.run(dir, args...)
	}

	err := RefuseMissingObjects(run, "repo", "--upload-pack=touch")

	if err != nil || asked {
		t.Errorf("RefuseMissingObjects = %v, rev-parse asked %v; want nil, and the name never handed to rev-parse", err, asked)
	}
}

func TestOnAnOldGitAQuestionAboutTheBaseThatIsNotAnswered(t *testing.T) {
	timedOut := &BoundError{Kind: BoundTimeout, message: "git execution timed out after 30s"}
	cases := []struct {
		name       string
		repository fakeRepository
	}{
		{"the base cannot be resolved in time", fakeRepository{revParseErr: timedOut}},
		{"git prints what is not an object id for the base", fakeRepository{revParseOutput: "not an id\n"}},
		{"the base's commit cannot be looked for", fakeRepository{probes: map[string]probeAnswer{firstMissing: {err: timedOut}}}},
	}
	for _, c := range cases {
		assertRefusedForWantOfAnAnswer(t, c.repository, "origin/main", "")
	}
}
