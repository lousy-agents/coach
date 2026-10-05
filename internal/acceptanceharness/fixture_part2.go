package acceptanceharness

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
