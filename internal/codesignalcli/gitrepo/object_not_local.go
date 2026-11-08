package gitrepo

import (
	"errors"
	"fmt"
	"os/exec"
)

// ObjectNotLocalError is an object that a tree names and the repository's object store lacks. In a partial
// clone that is an object left to be fetched on demand, which a read here never does.
type ObjectNotLocalError struct{ Object string }

func (e *ObjectNotLocalError) Error() string {
	return fmt.Sprintf("object %s is not in the local object store", e.Object)
}

// classifyObjectCheck is err, or an ObjectNotLocalError when err is the silent exit 1 that
// `git cat-file -e` ends with for an object it does not have. An invalid object is reported on stderr and
// so arrives as a different error.
func classifyObjectCheck(object string, err error) error {
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return &ObjectNotLocalError{Object: object}
	}
	return err
}

// ObjectsNotLocalError is the objects of a tree that the repository's object store lacks, found while
// reading the whole tree. It names how many and the first path, which is enough to tell the user what is
// missing without listing a clone's worth of paths.
type ObjectsNotLocalError struct {
	Count     int
	FirstPath string
	// Advice is how to fetch them in a partial clone, or empty.
	Advice string
}

func (e *ObjectsNotLocalError) Error() string {
	message := fmt.Sprintf("the revision has %s not in the local object store, and Coach does not fetch objects (the first is %q)", missingObjects{count: e.Count}.phrase(), e.FirstPath)
	if e.Advice != "" {
		message += "; " + e.Advice
	}
	return message
}

// ObjectsUnreadableError is the objects of a tree that the repository's object store holds and git cannot read:
// damaged, not missing. A fetch of what the store lacks does not replace them, so no fetch is offered; the message
// says what git said, and the check that finds the rest of the damage.
type ObjectsUnreadableError struct {
	Count     int
	FirstPath string
	Said      string
}

func (e *ObjectsUnreadableError) Error() string {
	count, are := fmt.Sprintf("%d objects", e.Count), "they are"
	if e.Count == 1 {
		count, are = "1 object", "it is"
	}
	return fmt.Sprintf("the revision has %s that git cannot read; %s in the object store and damaged, not missing (the first is %q); git said: %s; check the repository with git fsck and restore the damaged objects from a good copy of it", count, are, e.FirstPath, e.Said)
}
