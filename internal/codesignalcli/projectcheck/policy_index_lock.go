package projectcheck

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// indexIsLocked is whether another git process holds the index, which git marks with a lock file next to it. Where git
// cannot say where the lock would be, the answer is an error.
func indexIsLocked(run gitrepo.Runner, dir string) (bool, error) {
	output, err := run(dir, "rev-parse", "--git-path", "index.lock")
	if err != nil {
		return false, err
	}
	lock := strings.TrimSpace(string(output))
	if lock == "" {
		return false, errors.New("git printed no path for the index lock")
	}
	if !filepath.IsAbs(lock) {
		lock = filepath.Join(dir, lock)
	}
	_, statErr := os.Lstat(lock)
	return statErr == nil, nil
}
