package projectmodel

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
