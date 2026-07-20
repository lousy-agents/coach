package acceptanceharness

import "sync"

// FixtureSchemaVersion is the current version of the fixture/request-record
// schema this package defines. A fixture-driven fake service (Task 0.2's
// fake GitHub, and any later fixture-driven fake) must stamp every fixture
// file and RequestRecord it produces with this version, so a future schema
// change is visible and existing golden fixtures can be migrated
// deliberately rather than silently reinterpreted under a new shape.
const FixtureSchemaVersion = 1

// AuthMode identifies which credential (if any) a recorded request used.
// This is the shared vocabulary the epic's binding contract requires for
// proving GitHub-boundary token separation: OAuth access tokens must
// appear only on identity/login paths, repository reads and authorization
// must use GitHub App installation credentials, and a request that used
// neither -- or an incorrectly-scoped one -- must be recorded as
// AuthModeNone or AuthModeRejected rather than silently omitted from the
// record.
type AuthMode string

const (
	// AuthModeOAuth marks a request authenticated with a GitHub OAuth
	// access token (identity/login paths only).
	AuthModeOAuth AuthMode = "oauth"
	// AuthModeInstallation marks a request authenticated with a GitHub
	// App installation token (repository reads/authorization).
	AuthModeInstallation AuthMode = "installation"
	// AuthModeNone marks a request that carried no credential.
	AuthModeNone AuthMode = "none"
	// AuthModeRejected marks a request the fake/recorder refused --
	// including a misuse the epic calls out explicitly: an OAuth token
	// used against a repository endpoint, or an installation credential
	// used where an OAuth token belongs.
	AuthModeRejected AuthMode = "rejected"
)

// RequestRecord is the minimum shared shape a fixture-driven fake service
// must record for every request it handles, so a consuming acceptance test
// can assert the exact sequence, fixture/scenario, and authentication mode
// of calls made against it. Per the epic: "Request recording is part of
// the contract, not debug logging."
type RequestRecord struct {
	SchemaVersion int      `json:"schema_version"`
	Scenario      string   `json:"scenario"`
	Method        string   `json:"method"`
	Path          string   `json:"path"`
	AuthMode      AuthMode `json:"auth_mode"`
}

// NewRequestRecord builds a RequestRecord stamped with the current
// FixtureSchemaVersion, so callers can't accidentally omit or mismatch it.
func NewRequestRecord(scenario, method, path string, mode AuthMode) RequestRecord {
	return RequestRecord{
		SchemaVersion: FixtureSchemaVersion,
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
