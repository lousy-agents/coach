package gitrepo

import (
	"fmt"
	"strings"
)

// refuseWhenTheBaseCommitIsNotHere is the question about --base that comes before the merge base, on a git that
// cannot be told not to fetch. The merge base is found by reading the commits from the base and from HEAD, and
// that git fetches a commit it does not have: asking for it first ran the remote's transport and wrote objects, and
// only then refused. So the base is resolved and its commit looked for with questions that read no object, and the
// merge base is asked for only once the commit is here.
//
// A base that does not resolve is the scan's to word, as for every other failure of the revision it names.
func refuseWhenTheBaseCommitIsNotHere(run Runner, dir, base, version string) error {
	commit, err := baseCommitWithoutFetching(run, dir, base)
	if err != nil {
		return cannotTellWhatIsMissing(version, err)
	}
	if commit == "" {
		return nil
	}
	missing, err := commitMissingOf(run, dir, commit)
	if err != nil {
		return cannotTellWhatIsMissing(version, err)
	}
	if !missing.any() {
		return nil
	}
	revision := readRevision{name: commit, label: "the commit --base names"}
	return lacksObjects(run, dir, version, revision, missing, []string{commit, "HEAD"})
}

// baseCommitWithoutFetching is the object id that --base resolves to, and "" for a baseline and for a name that
// does not resolve. Resolving a name reads the ref, and an object id as it stands, and not the object: the
// question is not peeled to a commit, which would read it. A name that begins with a dash is not asked, because
// rev-list and rev-parse would read it as an option on a git too old for --end-of-options; the merge base's own
// question has a "--" before it, and the scan words what that says.
func baseCommitWithoutFetching(run Runner, dir, base string) (string, error) {
	if base == "" || strings.HasPrefix(base, "-") {
		return "", nil
	}
	output, err := run(dir, "rev-parse", "--verify", "--quiet", base)
	if gitRanAndFailed(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	commit := strings.TrimSpace(string(output))
	if !objectID.MatchString(commit) {
		return "", fmt.Errorf("git printed %q where the object id of --base was expected", commit)
	}
	return commit, nil
}
