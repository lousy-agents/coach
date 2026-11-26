package evalhistory

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

// ControlsPerFlagged is k, the controls drawn for each flagged unit.
const ControlsPerFlagged = 3

// Run checks out every corpus repository and checks its history range
// before scanning any, so an unusable input stops the run with nothing
// scanned. It then compares each Go rule's flagged units with their
// controls, repository by repository.
func Run(corpus evalcensus.Corpus, coach string, seed int64) (Summary, error) {
	if err := refuseUnusable(corpus); err != nil {
		return Summary{}, err
	}
	identity, err := evalcensus.IdentifyCoach(coach)
	if err != nil {
		return Summary{}, err
	}
	workRoot, err := os.MkdirTemp("", "coach-eval-history")
	if err != nil {
		return Summary{}, err
	}
	defer os.RemoveAll(workRoot)
	dirs, err := checkoutRanges(corpus, workRoot)
	if err != nil {
		return Summary{}, err
	}
	summary := Summary{
		CorpusID: corpus.ID, Coach: identity, Seed: seed, ControlsPerFlagged: ControlsPerFlagged,
		FixPattern: FixPattern.String(), Statement: Statement,
	}
	for i, repo := range corpus.Repositories {
		summary.Repositories = append(summary.Repositories, repositoryHistory(corpus, repo, identity.Path, dirs[i], seed))
	}
	return summary, nil
}

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

func checkoutRanges(corpus evalcensus.Corpus, workRoot string) ([]string, error) {
	dirs := make([]string, len(corpus.Repositories))
	for i, repo := range corpus.Repositories {
		dirs[i] = filepath.Join(workRoot, fmt.Sprintf("%02d", i))
		if err := evalcensus.CheckoutRepository(repo, dirs[i]); err != nil {
			return nil, err
		}
		if err := checkRange(repo, dirs[i]); err != nil {
			return nil, err
		}
	}
	return dirs, nil
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
