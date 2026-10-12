package coachapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Task 1: initial SQL migration defines jobs / job_findings / job_diagnostics
// with creator fields, attempt scoping, and NULLS NOT DISTINCT uniqueness.
func TestInitMigration_DefinesJobTables(t *testing.T) {
	path := filepath.Join("migrations", "0001_init.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s must not fail: %v", path, err)
	}
	sql := string(body)
	for _, frag := range []string{
		"CREATE TABLE jobs",
		"created_by_provider",
		"created_by_subject",
		"created_by_login",
		"attempt",
		"CREATE TABLE job_findings",
		"payload_hash",
		"NULLS NOT DISTINCT",
		"CREATE TABLE job_diagnostics",
		// Claim/reclaim and report assembly access paths (Task 3 builds on these).
		"CREATE INDEX job_findings_job_id_idx",
		"CREATE INDEX job_diagnostics_job_id_idx",
		"CREATE INDEX jobs_status_created_at_idx",
		"CREATE INDEX jobs_running_heartbeat_at_idx",
		// Story 5 provenance invariant at the DB boundary.
		"job_findings_provenance_chk",
	} {
		if !strings.Contains(sql, frag) {
			t.Errorf("migrations/0001_init.sql must contain %q", frag)
		}
	}
}
