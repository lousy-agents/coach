package evalharness

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
)

func run(dir string, env []string, stdin, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = strings.NewReader(stdin)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	return output.String(), err
}

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
