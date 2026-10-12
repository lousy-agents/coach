package baseline_test

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/baseline"
)

// uuidShape matches the UUID PRIMARY KEY shape Postgres job_findings/job_diagnostics use.
var uuidShape = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// captureWriter records fenced writes and enforces the Postgres invariants the
// real store would reject: non-empty UUID primary keys and UNIQUE NULLS NOT
// DISTINCT (job_id, attempt, source, rubric_id, payload_hash).
type captureWriter struct {
	lease       coachapi.ClaimLease
	findings    []coachapi.JobFinding
	diagnostics []coachapi.JobDiagnostic
}

var _ baseline.JobWriter = (*captureWriter)(nil)

func newCaptureWriter() *captureWriter {
	return &captureWriter{
		lease: coachapi.ClaimLease{
			JobID:    "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
			WorkerID: "baseline-test-worker",
			Attempt:  1,
		},
	}
}

func (w *captureWriter) Lease() coachapi.ClaimLease { return w.lease }

func (w *captureWriter) InsertDiagnostics(_ context.Context, diagnostics []coachapi.JobDiagnostic) error {
	seenID := map[string]struct{}{}
	for _, existing := range w.diagnostics {
		if existing.ID != "" {
			seenID[existing.ID] = struct{}{}
		}
	}
	for _, d := range diagnostics {
		if d.ID == "" {
			return errors.New("coachapi: job_diagnostics.id must be a non-empty UUID (postgres UUID PRIMARY KEY)")
		}
		if !uuidShape.MatchString(d.ID) {
			return fmt.Errorf("coachapi: job_diagnostics.id %q is not UUID-shaped", d.ID)
		}
		if _, dup := seenID[d.ID]; dup {
			return fmt.Errorf("coachapi: duplicate job_diagnostics.id %q", d.ID)
		}
		seenID[d.ID] = struct{}{}
	}
	w.diagnostics = append(w.diagnostics, diagnostics...)
	return nil
}
