package evalhistory

import (
	"fmt"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

// refuseUnusable refuses a repository with no history range, and a
// TypeScript repository, which Coach would scan by running the repository's
// own compiler on the host until the pinned image (#832) exists.
func refuseUnusable(corpus evalcensus.Corpus) error {
	for _, repo := range corpus.Repositories {
		if repo.HistoryUntil == "" {
			return fmt.Errorf("repository %q has no history_until; pin the last commit the history reads", repo.ID)
		}
		if repo.Language == "typescript" {
			return fmt.Errorf("repository %q is TypeScript, which the history scans only inside the pinned image (#832), because scanning it on the host would run its own compiler", repo.ID)
		}
	}
	return nil
}

func checkRange(repo evalcensus.Repository, dir string) error {
	if _, err := git(dir, "cat-file", "-e", repo.HistoryUntil+"^{commit}"); err != nil {
		return fmt.Errorf("repository %q: history_until %s is not a commit in %s", repo.ID, repo.HistoryUntil, repo.Path)
	}
	if _, err := git(dir, "merge-base", "--is-ancestor", repo.SHA, repo.HistoryUntil); err != nil {
		return fmt.Errorf("repository %q: history_until %s does not descend from the census SHA %s", repo.ID, repo.HistoryUntil, repo.SHA)
	}
	return nil
}
