package coachapi

// SetFenceHoldForTest installs a hook invoked before atomic Postgres fenced
// inserts. Acceptance tests use it to let ClaimJob reclaim commit while an
// InsertFindings/InsertDiagnostics transaction is open, proving the insert
// fails with ErrClaimLost rather than winning a check-then-act race.
// Pass nil to clear.
func SetFenceHoldForTest(fn func()) {
	fenceHoldForTest = fn
}
