package acceptanceharness

import (
	"sync"
)

// RequestRecord is the minimum shared shape a fixture-driven fake service
// must record for every request it handles, so a consuming acceptance test
// can assert the exact sequence, fixture/scenario, and authentication mode
// of calls made against it. Per the epic: "Request recording is part of
// the contract, not debug logging."
//
// FixtureID identifies which fixture file/dataset was selected to serve the
// request, independent of Scenario (which names the specific behavior
// within that fixture, e.g. "not-found"). Two different fixtures can
// legitimately share a scenario name -- FixtureID is what keeps them
// distinguishable in a recorded request.
type RequestRecord struct {
	SchemaVersion int      `json:"schema_version"`
	FixtureID     string   `json:"fixture_id"`
	Scenario      string   `json:"scenario"`
	Method        string   `json:"method"`
	Path          string   `json:"path"`
	AuthMode      AuthMode `json:"auth_mode"`
}

// NewRequestRecord builds a RequestRecord stamped with the current
// FixtureSchemaVersion, so callers can't accidentally omit or mismatch it.
// fixtureID identifies which fixture file/dataset was selected, independent
// of scenario (which names the specific behavior within that fixture, e.g.
// "not-found") -- two different fixtures can legitimately share a scenario
// name, and fixtureID is what keeps them distinguishable in the recorded
// request.
func NewRequestRecord(fixtureID, scenario, method, path string, mode AuthMode) RequestRecord {
	return RequestRecord{
		SchemaVersion: FixtureSchemaVersion,
		FixtureID:     fixtureID,
		Scenario:      scenario,
		Method:        method,
		Path:          path,
		AuthMode:      mode,
	}
}

// Recorder is a minimal, concurrency-safe, append-only log of
// RequestRecords that a fixture-driven fake service should embed (or wrap)
// to satisfy the shared request-recording contract, rather than each fake
// inventing its own recording shape. Modeled on this package's existing
// GuardedTransport.BlockedRequests() pattern.
type Recorder struct {
	mu      sync.Mutex
	records []RequestRecord
}

// Record appends rec to the log, safe for concurrent use.
func (r *Recorder) Record(rec RequestRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, rec)
}

// Records returns a defensive copy of every RequestRecord recorded so far,
// in insertion order. Mutating the returned slice never affects the
// Recorder's internal state.
func (r *Recorder) Records() []RequestRecord {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]RequestRecord, len(r.records))
	copy(out, r.records)
	return out
}
