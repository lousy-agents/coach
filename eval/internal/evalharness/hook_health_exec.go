package evalharness

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
)

// isolatedGitEnv is the host environment without any GIT_* setting, and with no global or system git configuration:
// the check commits into a copy, so a GIT_DIR from a calling pre-commit hook, a host hooks path or a signing program
// shall not reach it.
func isolatedGitEnv() []string {
	env := []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			env = append(env, entry)
		}
	}
	return env
}

func run(dir string, env []string, stdin, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(isolatedGitEnv(), env...)
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
