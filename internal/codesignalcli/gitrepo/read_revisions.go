package gitrepo

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/shellword"
)

var objectID = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

// A revision the scan reads: the name git is asked for, and what a message calls it.
type readRevision struct {
	name  string
	label string
}

// revisionsRead are the commits whose trees the scan reads, in the order a fetch lists them: for a diff the merge
// base of base and HEAD, which is the other side of the changed files, then HEAD. Both branches of the guard ask
// this, so that they cannot disagree about what the scan reads.
func revisionsRead(run Runner, dir, base string) []readRevision {
	head := readRevision{name: "HEAD", label: "HEAD"}
	if mergeBase, found := mergeBaseRevisionOf(run, dir, base); found {
		return []readRevision{mergeBase, head}
	}
	return []readRevision{head}
}

// mergeBaseRevisionOf is the merge base of base and HEAD as a revision the scan reads. A baseline has no base,
// and a base that has no merge base is a failure the scan ends on and words itself, so neither has a revision
// to ask about.
func mergeBaseRevisionOf(run Runner, dir, base string) (readRevision, bool) {
	if base == "" {
		return readRevision{}, false
	}
	mergeBase, err := mergeBaseOf(run, dir, base)
	if err != nil {
		return readRevision{}, false
	}
	return mergeBaseRevision(base, mergeBase), true
}

func revisionNames(revisions []readRevision) []string {
	names := make([]string, len(revisions))
	for i, revision := range revisions {
		names[i] = revision.name
	}
	return names
}

// mergeBaseOf is the object id of the merge base of base and HEAD.
func mergeBaseOf(run Runner, dir, base string) (string, error) {
	output, err := run(dir, "merge-base", "--", base, "HEAD")
	if err != nil {
		return "", err
	}
	mergeBase := strings.TrimSpace(string(output))
	if !objectID.MatchString(mergeBase) {
		return "", fmt.Errorf("git printed %q where the merge base's object id was expected", mergeBase)
	}
	return mergeBase, nil
}

func mergeBaseRevision(base, mergeBase string) readRevision {
	return readRevision{name: mergeBase, label: "the merge base of " + displayed(base) + " and HEAD"}
}

func displayed(base string) string {
	if shellword.Printable(base) {
		return base
	}
	return "--base"
}
