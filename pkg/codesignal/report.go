package codesignal

import "github.com/lousy-agents/coach/pkg/semantics"

// Report is the top-level output of a Builder.Build call.
type Report struct {
	SchemaVersion string       `json:"schema_version"`
	Scope         Scope        `json:"scope"`
	Summary       Summary      `json:"summary"`
	Signals       []Signal     `json:"signals,omitempty"`
	Diagnostics   []Diagnostic `json:"diagnostics,omitempty"`
	Coverage      *Coverage    `json:"coverage,omitempty"`
}

// Summary counts files and signals across a Report.
type Summary struct {
	FilesAnalyzed        int `json:"files_analyzed"`
	FilesWithDiagnostics int `json:"files_with_diagnostics"`
	ActiveSignals        int `json:"active_signals"`
	IntroducedSignals    int `json:"introduced_signals"`
	ExistingSignals      int `json:"existing_signals"`
	ResolvedSignals      int `json:"resolved_signals"`
	BaselineSignals      int `json:"baseline_signals"`
}

type Category string

type Severity string

type Confidence string

type Lifecycle string

// Signal is one observation derived from a FileChange.
type Signal struct {
	ID             string             `json:"id"`
	Fingerprint    string             `json:"fingerprint"`
	RuleID         string             `json:"rule_id"`
	RuleVersion    string             `json:"rule_version"`
	Kind           string             `json:"kind"`
	Category       Category           `json:"category"`
	Severity       Severity           `json:"severity"`
	Confidence     Confidence         `json:"confidence"`
	Lifecycle      Lifecycle          `json:"lifecycle"`
	Changed        bool               `json:"changed"`
	Path           string             `json:"path"`
	SourceScope    string             `json:"source_scope,omitempty"`
	Subject        string             `json:"subject,omitempty"`
	Location       semantics.Location `json:"location"`
	Evidence       string             `json:"evidence,omitempty"`
	WhyItMatters   string             `json:"why_it_matters,omitempty"`
	Recommendation string             `json:"recommendation,omitempty"`
	SuggestedSkill string             `json:"suggested_skill,omitempty"`
	Provenance     Provenance         `json:"provenance"`
}

// Provenance records what produced a Signal.
type Provenance struct {
	Producer    string `json:"producer"`
	FindingKind string `json:"finding_kind,omitempty"`
}
