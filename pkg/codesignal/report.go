package codesignal

import "github.com/lousy-agents/coach/pkg/semantics"

// Report is the top-level output of a Builder.Build call.
type Report struct {
	SchemaVersion string       `json:"schema_version"`
	Scope         Scope        `json:"scope"`
	Summary       Summary      `json:"summary"`
	Signals       []Signal     `json:"signals,omitempty"`
	Diagnostics   []Diagnostic `json:"diagnostics,omitempty"`
}

// Summary counts files and signals across a Report.
type Summary struct {
	FilesAnalyzed        int `json:"files_analyzed"`
	FilesWithDiagnostics int `json:"files_with_diagnostics"`
	ActiveSignals        int `json:"active_signals"`
	IntroducedSignals    int `json:"introduced_signals"`
	ExistingSignals      int `json:"existing_signals"`
	ResolvedSignals      int `json:"resolved_signals"`
}

// Category classifies the kind of concern a Signal represents. The only
// value that exists in v1 is "state_management".
type Category string

// Severity is how impactful a Signal is judged to be. The only values that
// exist in v1 are "low", "medium", and "high".
type Severity string

// Confidence is how certain the rule that produced a Signal is. The only
// values that exist in v1 are "low", "medium", and "high".
type Confidence string

// Lifecycle classifies a Signal relative to Base/Head. The only values
// that exist in v1 are "introduced", "existing", "resolved", and
// "unknown".
type Lifecycle string

// Signal is one coaching-relevant observation derived from a FileChange.
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
