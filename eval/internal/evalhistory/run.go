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
