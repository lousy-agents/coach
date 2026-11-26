package evalcensus

// analysisFailed reports whether any repository's analysis failed, whether
// coach exited non-zero or its report could not be read.
func (s Summary) analysisFailed() bool {
	for _, repo := range s.Repositories {
		if repo.CoachExit != 0 || hasFailedRule(repo.Statuses) {
			return true
		}
	}
	return false
}

func hasFailedRule(statuses []RuleStatus) bool {
	for _, status := range statuses {
		if status.Status == AnalysisFailed {
			return true
		}
	}
	return false
}
