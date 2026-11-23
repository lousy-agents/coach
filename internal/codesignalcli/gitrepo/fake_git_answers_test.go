package gitrepo

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// answers is what a fake git says to each question the guard asks, keyed by its arguments.
type answers map[string]struct {
	output string
	err    error
}

func (a answers) run(_ string, args ...string) ([]byte, error) {
	answer, ok := a[strings.Join(args, " ")]
	if !ok {
		return nil, errors.New("unexpected question: " + strings.Join(args, " "))
	}
	return []byte(answer.output), answer.err
}

const (
	insideQuestion  = "rev-parse --is-inside-work-tree"
	layoutQuestion  = "rev-parse --absolute-git-dir --git-common-dir"
	worktreeSetting = "config --get core.worktree"
)

func makeDirectories(t *testing.T, dirs ...string) {
	t.Helper()
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}
