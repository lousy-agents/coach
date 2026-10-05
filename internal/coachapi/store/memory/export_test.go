package memory

// StoredAttemptRowCountsForTest returns how many finding and diagnostic rows
// Store still holds for jobID across all attempts. Used by reclaim
// acceptance tests so "delete prior findings/diagnostics" is observed
// directly — GetReport alone filters by final attempt and would false-green
// if ClaimJob only incremented attempt without clearing prior rows.
func (m *Store) StoredAttemptRowCountsForTest(jobID string) (findings, diagnostics int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.jobs[jobID]
	if !ok {
		return 0, 0
	}
	return len(record.findings), len(record.diagnostics)
}
