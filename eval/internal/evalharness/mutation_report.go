package evalharness

import (
	"encoding/json"
	"fmt"
	"path"
	"slices"
)

// Mutant statuses as Gremlins reports them (design §6, step 6): KILLED and TIMED OUT are detected, LIVED and NOT
// COVERED are undetected, and NOT VIABLE and SKIPPED are not valid mutants at all.
const (
	statusKilled     = "KILLED"
	statusTimedOut   = "TIMED OUT"
	statusLived      = "LIVED"
	statusNotCovered = "NOT COVERED"
)

// The thresholds are our choice (design §6, step 6): the score is at least 0.8, and not covered is at most 10%
// of the valid mutants.
const (
	minimumScore       = 0.8
	maximumNotCovered  = 0.10
	thresholdTolerance = 1e-9
)

// MutationCounts is what a Gremlins report holds for the files of the api surface's route.
type MutationCounts struct {
	Killed, TimedOut, Lived, NotCovered int
}

// MutationVerdict is the gate's judgement of one run. Score and NotCoveredShare are nil when no mutant is
// valid, in which case the gate passes vacuously.
type MutationVerdict struct {
	Valid           int
	Score           *float64
	NotCoveredShare *float64
	Pass            bool
	Reason          string
}

type gremlinsReport struct {
	Files []struct {
		FileName  string `json:"file_name"`
		Mutations []struct {
			Status string `json:"status"`
		} `json:"mutations"`
	} `json:"files"`
}

// ParseMutationReport counts the mutants in the files whose directory is one of routeDirs, the packages
// reachable from the api surface.
func ParseMutationReport(data []byte, routeDirs []string) (MutationCounts, error) {
	var parsed gremlinsReport
	if err := json.Unmarshal(data, &parsed); err != nil {
		return MutationCounts{}, fmt.Errorf("the gremlins report is not JSON: %w", err)
	}
	var counts MutationCounts
	for _, file := range parsed.Files {
		if !slices.Contains(routeDirs, path.Dir(file.FileName)) {
			continue
		}
		for _, mutation := range file.Mutations {
			counts.add(mutation.Status)
		}
	}
	return counts, nil
}

func (c *MutationCounts) add(status string) {
	switch status {
	case statusKilled:
		c.Killed++
	case statusTimedOut:
		c.TimedOut++
	case statusLived:
		c.Lived++
	case statusNotCovered:
		c.NotCovered++
	}
}

// Verdict applies the thresholds: the score is detected over valid, and not covered is judged as a share of valid.
func (c MutationCounts) Verdict() MutationVerdict {
	valid := c.Killed + c.TimedOut + c.Lived + c.NotCovered
	if valid == 0 {
		return MutationVerdict{Pass: true}
	}
	score := float64(c.Killed+c.TimedOut) / float64(valid)
	share := float64(c.NotCovered) / float64(valid)
	verdict := MutationVerdict{Valid: valid, Score: &score, NotCoveredShare: &share, Pass: true}
	if score < minimumScore-thresholdTolerance {
		verdict.Pass, verdict.Reason = false, fmt.Sprintf("the mutation score is %.2f, below %.2f", score, minimumScore)
	} else if share > maximumNotCovered+thresholdTolerance {
		verdict.Pass, verdict.Reason = false, fmt.Sprintf("not covered is %.0f%% of the valid mutants, above %.0f%%", share*100, maximumNotCovered*100)
	}
	return verdict
}
