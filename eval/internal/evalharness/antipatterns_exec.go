package evalharness

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
)

// runIn runs a program in dir with extra environment entries and standard input, and returns its combined output.
func runIn(dir string, env []string, stdin, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = strings.NewReader(stdin)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	return output.String(), err
}

// antipatternExit is a command's exit status: 0 for no error, -1 for a failure to run it.
func antipatternExit(err error) int {
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exitErr):
		return exitErr.ExitCode()
	}
	return -1
}
