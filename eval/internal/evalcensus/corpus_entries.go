package evalcensus

import "fmt"

func repositoriesProblem(repositories []Repository) string {
	seen := map[string]bool{}
	for _, repo := range repositories {
		if problem := repositoryProblem(repo); problem != "" {
			return fmt.Sprintf("repository %q: %s", repo.ID, problem)
		}
		if seen[repo.ID] {
			return "duplicate repository id " + repo.ID
		}
		seen[repo.ID] = true
	}
	return ""
}

func repositoryProblem(repo Repository) string {
	switch {
	case repo.ID == "":
		return "id is empty"
	case repo.Path == "":
		return "path is empty"
	case !shaPattern.MatchString(repo.SHA):
		return "sha is not 40 lowercase hex digits; pin a commit, not a ref"
	case !languages[repo.Language]:
		return "language must be go or typescript"
	case repo.HistoryUntil != "" && !shaPattern.MatchString(repo.HistoryUntil):
		return "history_until is not 40 lowercase hex digits; pin a commit, not a ref"
	}
	return ""
}

func rulesProblem(rules []Rule) string {
	seen := map[string]bool{}
	for _, rule := range rules {
		if problem := ruleProblem(rule); problem != "" {
			return fmt.Sprintf("rule %q: %s", rule.RuleID, problem)
		}
		if seen[rule.RuleID] {
			return "duplicate rule " + rule.RuleID
		}
		seen[rule.RuleID] = true
	}
	return ""
}

func ruleProblem(rule Rule) string {
	if rule.RuleID == "" {
		return "rule_id is empty"
	}
	if len(rule.Languages) == 0 {
		return "languages is empty"
	}
	for _, language := range rule.Languages {
		if !languages[language] {
			return "unknown language " + language
		}
	}
	return ""
}
