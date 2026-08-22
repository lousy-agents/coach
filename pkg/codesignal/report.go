package codesignal

import (
	"encoding/json"

	"github.com/lousy-agents/coach/pkg/projectmodel"
	"github.com/lousy-agents/coach/pkg/semantics"
)

type Report struct {
	SchemaVersion string       `json:"schema_version"`
	Scope         Scope        `json:"scope"`
	Summary       Summary      `json:"summary"`
	Signals       []Signal     `json:"signals"`
	Diagnostics   []Diagnostic `json:"diagnostics"`
	Coverage      *Coverage    `json:"coverage"`

	ProjectChanges  []ProjectChange        `json:"project_changes"`
	ProjectFacts    []ProjectFact          `json:"project_facts"`
	ProjectSummary  *ProjectSummary        `json:"project_summary"`
	ProjectCoverage *projectmodel.Coverage `json:"project_coverage"`
}

type reportWireV1 struct {
	SchemaVersion string       `json:"schema_version"`
	Scope         Scope        `json:"scope"`
	Summary       Summary      `json:"summary"`
	Signals       []Signal     `json:"signals"`
	Diagnostics   []Diagnostic `json:"diagnostics"`
	Coverage      Coverage     `json:"coverage"`
}

type reportWireV2 struct {
	reportWireV1
	ProjectChanges  []ProjectChange        `json:"project_changes"`
	ProjectFacts    []ProjectFact          `json:"project_facts"`
	ProjectSummary  *ProjectSummary        `json:"project_summary"`
	ProjectCoverage *projectmodel.Coverage `json:"project_coverage"`
}

func (r Report) MarshalJSON() ([]byte, error) {
	base := reportWireV1{
		SchemaVersion: r.SchemaVersion,
		Scope:         r.Scope,
		Summary:       r.Summary,
		Signals:       nonNilSlice(r.Signals),
		Diagnostics:   nonNilSlice(r.Diagnostics),
		Coverage:      nonNilCoverage(r.Coverage),
	}
	if r.SchemaVersion != "2" {
		return json.Marshal(base)
	}

	projectCoverage := r.ProjectCoverage
	if projectCoverage == nil {
		projectCoverage = &projectmodel.Coverage{}
	}
	projectSummary := r.ProjectSummary
	if projectSummary == nil {
		projectSummary = &ProjectSummary{}
	}
	return json.Marshal(reportWireV2{
		reportWireV1:    base,
		ProjectChanges:  nonNilSlice(r.ProjectChanges),
		ProjectFacts:    nonNilSlice(r.ProjectFacts),
		ProjectSummary:  projectSummary,
		ProjectCoverage: projectCoverage,
	})
}

func nonNilSlice[T any](in []T) []T {
	if in == nil {
		return []T{}
	}
	return in
}

func nonNilMap[K comparable, V any](in map[K]V) map[K]V {
	if in == nil {
		return map[K]V{}
	}
	return in
}

func nonNilCoverage(in *Coverage) Coverage {
	var out Coverage
	if in != nil {
		out = *in
	}
	out.Unsupported = nonNilSlice(out.Unsupported)
	out.Excluded = nonNilSlice(out.Excluded)
	return out
}

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
	WhyItMatters   string             `json:"why_it_matters"`
	Recommendation string             `json:"recommendation"`
	SuggestedSkill string             `json:"suggested_skill"`
	Provenance     Provenance         `json:"provenance"`

	MachineEvidence  map[string]string `json:"machine_evidence"`
	RelatedLocations []ProjectLocation `json:"related_locations"`
	PathSteps        []ProjectPathStep `json:"path_steps"`
	CoverageRefs     []string          `json:"coverage_refs"`
}

// signalWire is a defined type over Signal: it shares every field and JSON
// tag but drops Signal's method set, so json.Marshal(signalWire(s)) encodes
// via the default struct encoder instead of recursing into
// Signal.MarshalJSON.
type signalWire Signal

func (s Signal) MarshalJSON() ([]byte, error) {
	w := signalWire(s)
	w.MachineEvidence = nonNilMap(w.MachineEvidence)
	w.RelatedLocations = nonNilSlice(w.RelatedLocations)
	w.PathSteps = nonNilSlice(w.PathSteps)
	w.CoverageRefs = nonNilSlice(w.CoverageRefs)
	return json.Marshal(w)
}

type Provenance struct {
	Producer    string `json:"producer"`
	FindingKind string `json:"finding_kind,omitempty"`

	// Language is the source language a project-origin fact was derived
	// from ("go" or "typescript"), when the producer records one. It is
	// the only queryable cross-language provenance signal on ProjectFact,
	// which unlike ProjectChange has no MachineEvidence map to carry an
	// equivalent key (see ReachabilityProjectFacts).
	Language string `json:"language,omitempty"`
}
