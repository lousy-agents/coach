package projectconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// restoresFromTheCommit is whether a verdict ends in bringing back the committed copy, which is only worth
// doing when that copy is a valid policy.
func restoresFromTheCommit(verdict restoreVerdict) bool {
	return verdict == restoreOverwritesNothing || verdict == restoreOverwritesEmptyEntry
}

// indexIsLocked is whether the index lock file exists, which git leaves while another git process
// writes the index and after one died. Both forms of restore Coach prints write the index. Where git
// cannot say where the lock would be, it is an error, and the restore is declined like every other
// question git leaves unanswered.
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

// sparseCheckoutEnabled reports whether core.sparseCheckout is on. It asks git only when a skip-worktree
// entry makes the answer matter. An unset key, which git reports with exit 1, is off; any other failure is
// an error, because the advice for a skip-worktree entry is a different one in a sparse checkout.
func sparseCheckoutEnabled(run gitrepo.Runner, dir string, staged []gitrepo.IndexEntry) (bool, error) {
	if !skippedBySparseCheckout(staged) {
		return false, nil
	}
	out, err := run(dir, "config", "--type=bool", "--get", "core.sparseCheckout")
	if err != nil {
		if gitrepo.ConfigHasNoValue(err) {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(string(out)) == "true", nil
}
