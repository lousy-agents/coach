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
