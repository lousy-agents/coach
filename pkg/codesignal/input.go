package codesignal

import "github.com/lousy-agents/coach/pkg/semantics"

// Input is the unit of work for a Builder.
type Input struct {
	Scope       Scope        `json:"scope"`
	Files       []FileChange `json:"files,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
	Coverage    *Coverage    `json:"coverage,omitempty"`
}

// Scope identifies the repository and revision range an Input covers.
type Scope struct {
	Repository string `json:"repository,omitempty"`
	Revision   string `json:"revision,omitempty"`
	Base       string `json:"base,omitempty"`
	Baseline   bool   `json:"baseline,omitempty"`
}

type ChangeStatus string

// FileChange describes one file's before/after analysis results.
type FileChange struct {
	Path          string            `json:"path"`
	Status        ChangeStatus      `json:"status,omitempty"`
	SourceScope   string            `json:"source_scope,omitempty"`
	Base          *semantics.Result `json:"base,omitempty"`
	Head          *semantics.Result `json:"head,omitempty"`
	ChangedRanges []LineRange       `json:"changed_ranges,omitempty"`
}

// LineRange is a 0-based, inclusive row range.
type LineRange struct {
	StartRow uint `json:"start_row"`
	EndRow   uint `json:"end_row"`
}
