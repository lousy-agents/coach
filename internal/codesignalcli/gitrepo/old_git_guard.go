package gitrepo

import (
	"errors"
	"fmt"
	"os/exec"
)

// The refusals for a git before 2.44, which cannot be told not to fetch: a revision the scan reads that lacks
// objects is refused, and so is a question the probe cannot answer, because a read that might fetch is not made
// on a guess.

func refuseWhenReadRevisionsLackObjects(run Runner, dir, base, version string) error {
	revisions := revisionsRead(run, dir, base)
	for _, revision := range revisions {
		missing, err := missingObjectsOf(run, dir, revision.name)
		if err != nil {
			return unansweredProbe(run, dir, version, revision, err)
		}
		if missing.any() {
			return lacksObjects(run, dir, version, revision, missing, revisionNames(revisions))
		}
	}
	return nil
}

// unansweredProbe is a refusal only for a revision that exists. One that does not resolve (a HEAD with no commit
// behind it, a directory that is no repository) is a read the scan never makes: it ends on the revision and says
// why in its own words, as it does on a git that can be told not to fetch, and a message about the version of git
// would send the reader to upgrade it for a failure an upgrade does not touch.
func unansweredProbe(run Runner, dir, version string, revision readRevision, cause error) error {
	if gitSaysItNamesNothing(run, dir, revision.name) {
		return nil
	}
	return cannotTellWhatIsMissing(version, cause)
}

// gitSaysItNamesNothing is whether git finishes with a failure when asked to resolve name: an unborn HEAD, or no
// repository to resolve it in. A git that did not finish, or could not be started, says nothing about it.
func gitSaysItNamesNothing(run Runner, dir, name string) bool {
	_, err := run(dir, "rev-parse", "--verify", "--quiet", name)
	return gitRanAndFailed(err)
}

// lacksObjects words the refusal. The repository is called a partial clone only when the configuration Coach
// reads says so, and it is told the fetch only then: the refusal does not depend on it.
func lacksObjects(run Runner, dir, version string, revision readRevision, missing missingObjects, names []string) error {
	subject := ""
	if _, partial := PromisorRemote(run, dir); partial {
		subject = "this repository is a partial clone, "
	}
	return &OperationalError{Message: fmt.Sprintf("%sgit %s cannot be told not to fetch missing objects, and %s has %s not in it, so reading it could run its remote's transport and write objects; %s", subject, version, revision.label, missing.phrase(), wayOut(FetchAdviceFor(run, dir, names...)))}
}

// cannotTellWhatIsMissing is the refusal for a question the probe could not answer. Every question of the guard
// ends here, so this is where a git that is not installed is told apart: its version is "unknown" only because
// it cannot be started, and it is said so as every other read of git says it.
func cannotTellWhatIsMissing(version string, cause error) error {
	if errors.Is(cause, exec.ErrNotFound) {
		return gitNotFoundError()
	}
	return &OperationalError{Message: fmt.Sprintf("could not tell whether this repository has the objects Coach reads (%s), and git %s cannot be told not to fetch the objects a partial clone lacks, so a read could run its remote's transport and write objects; %s", TerminalSafe(cause.Error()), version, wayOut("")), Cause: cause}
}

// wayOut is the newer git that fixes the problem, and the fetch that brings the objects in when there is
// one that is safe to paste.
func wayOut(fetch string) string {
	upgrade := fmt.Sprintf("use git %d.%d or later", noLazyFetchMajor, noLazyFetchMinor)
	if fetch == "" {
		return upgrade
	}
	return upgrade + ", or " + fetch
}
