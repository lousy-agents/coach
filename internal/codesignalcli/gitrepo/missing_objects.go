package gitrepo

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/shellword"
)

// A revision the scan reads: the name git is asked for, and what a message calls it.
type readRevision struct {
	name  string
	label string
}

var (
	missingObjectLine = regexp.MustCompile(`^\?[0-9a-f]{40,64}$`)
	objectID          = regexp.MustCompile(`^[0-9a-f]{40,64}$`)
)

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

// missingObjects counts the objects of one revision's tree that are not in the repository. many is set
// when there are more than the output budget holds, which is an answer: the revision lacks objects.
type missingObjects struct {
	count int
	many  bool
}

func (m missingObjects) any() bool { return m.many || m.count > 0 }

// phrase is how a message says what is missing, in the grammar of its count.
func (m missingObjects) phrase() string {
	switch {
	case m.many:
		return "many objects that are"
	case m.count == 1:
		return "1 object that is"
	}
	return fmt.Sprintf("%d objects that are", m.count)
}

// missingObjectsOf asks git which objects of revision's own tree are not local. It reads no object it does
// not have: `--missing=print` lists a missing object where a walk would fetch it, and `--no-walk` keeps the
// listing to that one tree instead of the history behind it. `--quiet` leaves only the missing ones, so
// anything else on the output is a git that did not do as asked, and is an error.
func missingObjectsOf(run Runner, dir, revision string) (missingObjects, error) {
	return probeMissing(run, dir, revision)
}

// treesMissingOf is missingObjectsOf for the trees alone. `--filter=blob:none` leaves the blobs out of the
// listing, so they are not looked for and not reported, and a tree that is missing is: a clone made without
// trees lacks the root tree, and nothing under it can be listed until it is here.
func treesMissingOf(run Runner, dir, revision string) (missingObjects, error) {
	return probeMissing(run, dir, revision, "--filter=blob:none")
}

func probeMissing(run Runner, dir, revision string, filter ...string) (missingObjects, error) {
	args := append([]string{"rev-list", "--objects", "--missing=print", "--no-walk"}, filter...)
	output, err := run(dir, append(args, "--quiet", revision)...)
	var bound *BoundError
	if errors.As(err, &bound) && bound.Kind == BoundStdout {
		return missingObjects{many: true}, nil
	}
	if err != nil {
		return missingObjects{}, err
	}
	text := strings.TrimSuffix(string(output), "\n")
	if text == "" {
		return missingObjects{}, nil
	}
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if !missingObjectLine.MatchString(line) {
			return missingObjects{}, fmt.Errorf("git printed %q where a missing object's id was expected", line)
		}
	}
	return missingObjects{count: len(lines)}, nil
}
