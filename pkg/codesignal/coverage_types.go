package codesignal

// Coverage reports file-discovery accounting for a Repository Baseline run.
// A base-diff Report also sets Coverage, but only to carry Excluded (the
// diff flow's filtered-file tally, for the same "why did my file disappear"
// disclosure a baseline report gives); TrackedFilesDiscovered, FilesAnalyzed,
// and FilesUnanalyzable are full-repository accounting that only baseline
// mode computes, and read as zero on a diff-flow report regardless of how
// many files it actually analyzed -- see Report.Summary for diff-flow file
// counts instead. Coverage is nil on a diff-flow report when nothing was
// filtered.
type Coverage struct {
	TrackedFilesDiscovered int             `json:"tracked_files_discovered"`
	FilesAnalyzed          int             `json:"files_analyzed"`
	FilesUnanalyzable      int             `json:"files_unanalyzable"`
	Unsupported            []CoverageGroup `json:"unsupported,omitempty"`
	Excluded               []CoverageGroup `json:"excluded,omitempty"`
}

// CoverageGroup summarizes a set of files sharing an exclusion/unsupported
// reason and language or file-type, rather than one entry per file.
type CoverageGroup struct {
	Reason   string `json:"reason"`
	Language string `json:"language,omitempty"`
	Count    int    `json:"count"`
}
