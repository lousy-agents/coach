package evalhistory

import (
	"slices"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

// involvements is which fix first touched each function and each file.
type involvements struct {
	functions, files Involvement
}

// repositoryHistory scans the checkout at the census SHA, reads its fixes
// up to history_until, and compares each Go rule. A Coach or git failure
// marks the repository's analysis failed instead of stopping the run.
func repositoryHistory(corpus evalcensus.Corpus, repo evalcensus.Repository, coach, dir string, seed int64) RepositoryHistory {
	result := RepositoryHistory{ID: repo.ID, Since: repo.SHA, Until: repo.HistoryUntil}
	run := evalcensus.RunCoach(repo, coach, dir)
	if run.Failure != "" {
		result.CoachExit, result.Evidence = run.Exit, run.Failure
		return result
	}
	findings, err := decodeFindings(run.Report)
	if err != nil {
		result.Evidence = err.Error()
		return result
	}
	if err := result.compare(corpus, dir, findings, seed); err != nil {
		result.Evidence = err.Error()
	}
	return result
}

func (result *RepositoryHistory) compare(corpus evalcensus.Corpus, dir string, findings []finding, seed int64) error {
	inventory, err := GoInventory(dir, result.Since)
	if err != nil {
		return err
	}
	scan, err := FindFixes(dir, result.Since, result.Until)
	if err != nil {
		return err
	}
	involved, err := involveAll(dir, result.Since, inventory, scan.Fixes)
	if err != nil {
		return err
	}
	result.countFixes(scan, involved)
	for _, rule := range corpus.Rules {
		if !slices.Contains(rule.Languages, "go") {
			continue
		}
		ruleResult, placed := compareRule(inventory, involved, findings, rule.RuleID, seed)
		result.Rules = append(result.Rules, ruleResult)
		result.findings = append(result.findings, placed...)
	}
	return nil
}

func involveAll(dir, since string, inventory Inventory, fixes []FixCommit) (involvements, error) {
	functions, err := Involve(dir, since, inventory.Functions, fixes)
	if err != nil {
		return involvements{}, err
	}
	files, err := Involve(dir, since, inventory.Files, fixes)
	if err != nil {
		return involvements{}, err
	}
	return involvements{functions: functions, files: files}, nil
}

func (result *RepositoryHistory) countFixes(scan FixScan, involved involvements) {
	result.Commits, result.Fixes = scan.Commits, len(scan.Fixes)
	rate := 0.0
	if scan.Commits > 0 {
		rate = float64(len(scan.Fixes)) / float64(scan.Commits)
	}
	result.MatchRate = &rate
	result.UnmappedFixes = UnmappedFixes{Function: involved.functions.Unmapped, File: involved.files.Unmapped}
	result.UnprocessedFixes = involved.functions.Unprocessed
}
