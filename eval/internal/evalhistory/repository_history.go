package evalhistory

import (
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
	result := RepositoryHistory{ID: repo.ID, Since: repo.SHA, Until: repo.HistoryUntil, Rules: []RuleResult{}, UnprocessedFixes: []UnprocessedFix{}}
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
	ruleIDs, err := goRuleIDs(corpus)
	if err != nil {
		return err
	}
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
	rules, placed, err := compareRules(ruleIDs, inventory, involved, findings, seed)
	if err != nil {
		return err
	}
	result.Rules = append(result.Rules, rules...)
	result.findings = append(result.findings, placed...)
	return nil
}
