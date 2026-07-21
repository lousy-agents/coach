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
	// AuthModeRejected marks a refused credential: OAuth on a repo path,
	// installation token on an OAuth path, or a non-GitHub credential
	// (e.g. Coach JWT) sent to GitHub.
	AuthModeRejected AuthMode = "rejected"
)

// FixtureHeader is the shared, versioned envelope every fixture file a
// fixture-driven fake service (Task 0.2's fake GitHub, and later
// fixture-driven fakes) must embed at its top level. SchemaVersion pins the
// fixture to a specific FixtureSchemaVersion so a future schema change is
// visible and existing golden fixtures can be migrated deliberately rather
// than silently reinterpreted under a new shape (see "Golden fixture
// versioning" in docs/architecture/acceptance-harness.md section 3).
// FixtureID identifies which fixture file/dataset the header belongs to,
// independent of any scenario name a request against it might record --
// two different fixture files can legitimately share a scenario name (e.g.
// both defining a "not-found" behavior), and FixtureID is what keeps them
// distinguishable.
type FixtureHeader struct {
	SchemaVersion int    `json:"schema_version"`
	FixtureID     string `json:"fixture_id"`
}

// NewFixtureHeader builds a FixtureHeader stamped with the current
// FixtureSchemaVersion and the given fixtureID, so callers can't
// accidentally omit or mismatch the schema version.
func NewFixtureHeader(fixtureID string) FixtureHeader {
	return FixtureHeader{SchemaVersion: FixtureSchemaVersion, FixtureID: fixtureID}
}

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
