package coachapi

import (
	"encoding/json"
	"time"
)

// ReportVersion1 is the frozen groundwork-era report_version value.
const ReportVersion1 = "1"

// Report is the frozen snake_case job report contract (report_version "1").
// Error is always present: JSON null when the job succeeded, a string when it failed.
// Findings and Diagnostics always serialize as JSON arrays (never null); a nil
// Summary.FindingCounts serializes as {}.
type Report struct {
	ReportVersion string          `json:"report_version"`
	JobID         string          `json:"job_id"`
	Kind          JobKind         `json:"kind"`
	Params        json.RawMessage `json:"params"`
	CommitSHA     string          `json:"commit_sha"`
	Summary       ReportSummary   `json:"summary"`
	Findings      []Finding       `json:"findings"`
	Diagnostics   []Diagnostic    `json:"diagnostics"`
	Error         *string         `json:"error"`
	Versions      ReportVersions  `json:"versions"`
	GeneratedAt   time.Time       `json:"generated_at"`
}

// MarshalJSON keeps the frozen wire shape: nil slices become [] and a nil
// finding_counts map becomes {}, so producers cannot accidentally emit null
// for fields the contract documents as arrays/objects.
func (r Report) MarshalJSON() ([]byte, error) {
	type reportJSON Report
	out := reportJSON(r)
	if out.Findings == nil {
		out.Findings = []Finding{}
	}
	if out.Diagnostics == nil {
		out.Diagnostics = []Diagnostic{}
	}
	if out.Summary.FindingCounts == nil {
		out.Summary.FindingCounts = map[string]map[string]int{}
	}
	return json.Marshal(out)
}

// ReportSummary is a named-field summary (not a free-form map) so kind-specific
// counters cannot collide with rule/rubric ids in finding_counts.
type ReportSummary struct {
	FindingCounts map[string]map[string]int `json:"finding_counts"`
	PRCount       *int                      `json:"pr_count,omitempty"`
	PRFailedCount *int                      `json:"pr_failed_count,omitempty"`
}

// ReportVersions records analyzer and rubric id→version pairs used to build a report.
type ReportVersions struct {
	Analyzer string            `json:"analyzer"`
	Rubrics  map[string]string `json:"rubrics,omitempty"`
}

// Finding is one report finding with provenance fields.
// Deterministic findings carry null rubric_id, rubric_version, and model_identity.
type Finding struct {
	Source        FindingSource   `json:"source"`
	RubricID      *string         `json:"rubric_id"`
	RubricVersion *string         `json:"rubric_version"`
	ModelIdentity *string         `json:"model_identity"`
	Payload       json.RawMessage `json:"payload"`
}

// Diagnostic is one report diagnostic entry.
type Diagnostic struct {
	Scope   string `json:"scope"`
	Message string `json:"message"`
}
