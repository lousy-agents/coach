package codesignal

// Coverage reports file-discovery accounting for a Repository Baseline run.
// It is nil for a non-baseline (base-diff) Report.
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
