package domain

import (
	"sort"
)

// Coverage reports how completely a Model's facts were collected for a
// Snapshot, separate from and never referencing pkg/codesignal's own
// coverage/diagnostic types -- this is a project-model-native contract.
//
// Counts and Budgets are Go maps, but encoding/json sorts map keys
// alphabetically on marshal, so their JSON key order is deterministic
// regardless of insertion order; see acceptance_test.go for the test that
// pins this behavior.
type Coverage struct {
	Phase       string         `json:"phase"`
	Complete    bool           `json:"complete"`
	Counts      map[string]int `json:"counts,omitempty"`
	Budgets     map[string]int `json:"budgets,omitempty"`
	Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
}

// Diagnostic is a single project-analysis failure or limitation
// encountered while building a Model (e.g. an unresolved import).
type Diagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

// DiagBackendUnavailable must stay byte-identical to the
// "project_backend_unavailable" string embedded in
// internal/codesignalcli.ProjectBackendUnavailableError's message.
const DiagBackendUnavailable = "project_backend_unavailable"

// DiagRootScopeIncomplete marks a TypeScript root whose analyzed set does
// not cover every candidate file.
const DiagRootScopeIncomplete = "project_root_scope_incomplete"

func CanonicalCoverage(in Coverage) Coverage {
	if len(in.Diagnostics) == 0 {
		return in
	}
	out := in
	diagnostics := append([]Diagnostic(nil), in.Diagnostics...)
	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].Code != diagnostics[j].Code {
			return diagnostics[i].Code < diagnostics[j].Code
		}
		if diagnostics[i].Path != diagnostics[j].Path {
			return diagnostics[i].Path < diagnostics[j].Path
		}
		return diagnostics[i].Message < diagnostics[j].Message
	})
	out.Diagnostics = diagnostics
	return out
}
