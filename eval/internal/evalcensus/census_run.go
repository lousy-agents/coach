package evalcensus

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// Summary is one census run: the corpus, the Coach binary it scanned with,
// and each repository's rule statuses.
type Summary struct {
	CorpusID     string             `json:"corpus_id"`
	Coach        CoachIdentity      `json:"coach"`
	Repositories []RepositoryResult `json:"repositories"`
}

// CoachIdentity names the Coach binary a run used, so a re-run can match it.
type CoachIdentity struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// RepositoryResult is one repository's census outcome.
type RepositoryResult struct {
	ID        string       `json:"id"`
	SHA       string       `json:"sha"`
	CoachExit int          `json:"coach_exit"`
	Statuses  []RuleStatus `json:"statuses"`
}

func (s Summary) coachFailed() bool {
	for _, repo := range s.Repositories {
		if repo.CoachExit != 0 {
			return true
		}
	}
	return false
}

// Run checks out every corpus repository before scanning any, so an
// unreachable SHA stops the run with nothing scanned, then scans each.
func Run(corpus Corpus, coach string) (Summary, error) {
	identity, err := identifyCoach(coach)
	if err != nil {
		return Summary{}, err
	}
	workRoot, err := os.MkdirTemp("", "coach-eval-census")
	if err != nil {
		return Summary{}, err
	}
	defer os.RemoveAll(workRoot)
	dirs, err := checkoutAll(corpus, workRoot)
	if err != nil {
		return Summary{}, err
	}
	summary := Summary{CorpusID: corpus.ID, Coach: identity}
	for i, repo := range corpus.Repositories {
		summary.Repositories = append(summary.Repositories, scanRepository(corpus, repo, identity.Path, dirs[i]))
	}
	return summary, nil
}

func identifyCoach(coach string) (CoachIdentity, error) {
	raw, err := os.ReadFile(coach)
	if err != nil {
		return CoachIdentity{}, fmt.Errorf("--coach %s: %w", coach, err)
	}
	digest := sha256.Sum256(raw)
	absolute, err := filepath.Abs(coach)
	if err != nil {
		return CoachIdentity{}, err
	}
	return CoachIdentity{Path: absolute, SHA256: hex.EncodeToString(digest[:])}, nil
}

func checkoutAll(corpus Corpus, workRoot string) ([]string, error) {
	dirs := make([]string, len(corpus.Repositories))
	for i, repo := range corpus.Repositories {
		dirs[i] = filepath.Join(workRoot, fmt.Sprintf("%02d", i))
		if err := checkoutRepository(repo, dirs[i]); err != nil {
			return nil, err
		}
	}
	return dirs, nil
}
