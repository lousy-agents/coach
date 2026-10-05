package coachapi_test

import (
	"context"

	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/coachapi"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

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

func (f *fakeTreeSource) ReadFile(_ context.Context, _, _, ref, path string) ([]byte, string, error) {
	f.readCalls++
	f.lastReadRef = ref
	if f.readErr != nil {
		return nil, "", f.readErr
	}
	if f.contents == nil {
		return nil, "", githubingest.ErrNotFound
	}
	b, ok := f.contents[path]
	if !ok {
		return nil, "", githubingest.ErrNotFound
	}
	return append([]byte(nil), b...), "blob-sha", nil
}

func (w *memoryFencedWriter) InsertFindings(ctx context.Context, findings []coachapi.JobFinding) error {

	cap := &captureWriter{lease: w.lease}
	if err := cap.InsertFindings(ctx, findings); err != nil {
		return err
	}
	stamped := append([]coachapi.JobFinding(nil), findings...)
	for i := range stamped {
		stamped[i].JobID = w.lease.JobID
		stamped[i].Attempt = w.lease.Attempt
	}
	return w.store.InsertFindings(ctx, w.lease.JobID, w.lease.WorkerID, w.lease.Attempt, stamped)
}

func newCaptureWriter() *captureWriter {
	return &captureWriter{
		lease: coachapi.ClaimLease{
			JobID:    "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
			WorkerID: "baseline-test-worker",
			Attempt:  1,
		},
	}
}
