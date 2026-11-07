package gitrepo

import "fmt"

// The refusals for a git before 2.44, which cannot be told not to fetch: a revision the scan reads that lacks
// objects is refused, and so is a question the probe cannot answer, because a read that might fetch is not made
// on a guess.

func refuseWhenReadRevisionsLackObjects(run Runner, dir, base, version string) error {
	revisions, err := revisionsRead(run, dir, base)
	if err != nil {
		return cannotTellWhatIsMissing(version, err)
	}
	for _, revision := range revisions {
		missing, err := missingObjectsOf(run, dir, revision.name)
		if err != nil {
			return cannotTellWhatIsMissing(version, err)
		}
		if missing.any() {
			return lacksObjects(run, dir, version, revision, missing, revisionNames(revisions))
		}
	}
	return nil
}

func lacksObjects(run Runner, dir, version string, revision readRevision, missing missingObjects, names []string) error {
	return &OperationalError{Message: fmt.Sprintf("this repository is a partial clone, git %s cannot be told not to fetch missing objects, and %s has %s not in it, so reading it could run its remote's transport and write objects; %s", version, revision.label, missing.phrase(), wayOut(FetchAdviceFor(run, dir, names...)))}
}

func revisionNames(revisions []readRevision) []string {
	names := make([]string, len(revisions))
	for i, revision := range revisions {
		names[i] = revision.name
	}
	return names
}

func cannotTellWhatIsMissing(version string, cause error) error {
	return &OperationalError{Message: fmt.Sprintf("could not tell whether this partial clone has the objects Coach reads (%s), and git %s cannot be told not to fetch missing objects, so a read could run its remote's transport and write objects; %s", TerminalSafe(cause.Error()), version, wayOut("")), Cause: cause}
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
