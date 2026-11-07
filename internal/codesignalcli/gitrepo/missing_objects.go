package gitrepo

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	missingObjectLine = regexp.MustCompile(`^\?[0-9a-f]{40,64}$`)
)

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
