// Package gitrepo is the git process adapter behind coach codesignal:
// revision resolution, changed and tracked file selection, bounded and
// batched object reads, revision archives, and worktree status.
package gitrepo

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func Run(dir string, args ...string) (string, error) {
	output, err := RunBytes(dir, args...)
	return string(output), err
}

func RunBytes(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("%s: %s", err, strings.TrimSpace(stderr.String()))
		}
		return nil, err
	}
	return stdout.Bytes(), nil
}
