package baseline_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/coachapi"
)

func (w *captureWriter) InsertFindings(_ context.Context, findings []coachapi.JobFinding) error {
	seenID := map[string]struct{}{}
	seenUniq := map[string]struct{}{}
	for _, existing := range w.findings {
		if existing.ID != "" {
			seenID[existing.ID] = struct{}{}
		}
		seenUniq[findingUniqKey(existing)] = struct{}{}
	}
	for _, f := range findings {
		if f.ID == "" {
			return errors.New("coachapi: job_findings.id must be a non-empty UUID (postgres UUID PRIMARY KEY)")
		}
		if !uuidShape.MatchString(f.ID) {
			return fmt.Errorf("coachapi: job_findings.id %q is not UUID-shaped", f.ID)
		}
		if _, dup := seenID[f.ID]; dup {
			return fmt.Errorf("coachapi: duplicate job_findings.id %q", f.ID)
		}
		seenID[f.ID] = struct{}{}
		if f.PayloadHash == "" {
			return errors.New("coachapi: job_findings.payload_hash must be non-empty")
		}
		key := findingUniqKey(f)
		if _, dup := seenUniq[key]; dup {
			return fmt.Errorf("coachapi: duplicate job_findings unique key %s (UNIQUE NULLS NOT DISTINCT)", key)
		}
		seenUniq[key] = struct{}{}
	}
	w.findings = append(w.findings, findings...)
	return nil
}
