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
	message := fmt.Sprintf("%d objects are not in the local object store, and Coach does not fetch objects (the first is %q)", e.Count, e.FirstPath)
	if e.Advice != "" {
		message += "; " + e.Advice
	}
	return message
}
