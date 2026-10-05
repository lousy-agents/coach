package coachapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// GET /v1/jobs/{id} status body: error key is always present (null when unset),
// matching Report.error — not omitempty — so clients share one nullability rule.
func TestJobStatusResponse_ErrorSerializesAsNullWhenUnset(t *testing.T) {
	raw, err := json.Marshal(JobStatusResponse{
		ID:      "id",
		Kind:    JobKindRepoBaselineScan,
		Status:  JobStatusQueued,
		Attempt: 0,
		Error:   nil,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatalf("unmarshal map: %v", err)
	}
	errRaw, ok := asMap["error"]
	if !ok {
		t.Fatal(`JobStatusResponse JSON must include "error" key even when unset`)
	}
	if string(errRaw) != "null" {
		t.Errorf(`JobStatusResponse.error must be JSON null when unset; got %s`, errRaw)
	}
}

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

		"CREATE INDEX job_findings_job_id_idx",
		"CREATE INDEX job_diagnostics_job_id_idx",
		"CREATE INDEX jobs_status_created_at_idx",
		"CREATE INDEX jobs_running_heartbeat_at_idx",

		"job_findings_provenance_chk",
	} {
		if !strings.Contains(sql, frag) {
			t.Errorf("migrations/0001_init.sql must contain %q", frag)
		}
	}
}
