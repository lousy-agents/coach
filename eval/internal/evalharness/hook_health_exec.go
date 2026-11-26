package evalharness

import (
	"errors"
	"os/exec"
)

func exitCodeOf(err error) int {
	var exitErr *exec.ExitError
	if err == nil {
		return 0
	}
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}
