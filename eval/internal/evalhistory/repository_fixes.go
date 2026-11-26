package evalhistory

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
	if scan.Commits == 0 {
		result.MatchRateNullReason = "no commits between since and until"
	} else {
		rate := float64(len(scan.Fixes)) / float64(scan.Commits)
		result.MatchRate = &rate
	}
	result.UnmappedFixes = UnmappedFixes{Function: involved.functions.Unmapped, File: involved.files.Unmapped}
	result.UnprocessedFixes = append(result.UnprocessedFixes, involved.functions.Unprocessed...)
}
