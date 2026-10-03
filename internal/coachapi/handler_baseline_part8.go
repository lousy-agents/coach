package coachapi

func baselineCompletion(cfg RepoBaselineScanConfig, w BaselineJobWriter, commitSHA string) *Completion {
	now := cfg.Now().UTC()
	lease := w.Lease()
	return &Completion{
		Attempt:   lease.Attempt,
		CommitSHA: commitSHA,
		Versions: ReportVersions{
			Analyzer: baselineAnalyzerVersion,
			Rubrics:  seedRubricVersions(),
		},
		FinishedAt:  now,
		GeneratedAt: now,
	}
}
