package coachapi

// SetFenceHoldForTest installs a hook invoked before atomic Postgres fenced
// inserts. Acceptance tests use it to let ClaimJob reclaim commit while an
// InsertFindings/InsertDiagnostics transaction is open, proving the insert
// fails with ErrClaimLost rather than winning a check-then-act race.
// Pass nil to clear.
func SetFenceHoldForTest(fn func()) {
	fenceHoldForTest = fn
}

// StoredAttemptRowCountsForTest returns how many finding and diagnostic rows
// MemoryStore still holds for jobID across all attempts. Used by reclaim
// acceptance tests so "delete prior findings/diagnostics" is observed
// directly — GetReport alone filters by final attempt and would false-green
// if ClaimJob only incremented attempt without clearing prior rows.
func (m *MemoryStore) StoredAttemptRowCountsForTest(jobID string) (findings, diagnostics int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.jobs[jobID]
	if !ok {
		return 0, 0
	}
	return len(record.findings), len(record.diagnostics)
}
