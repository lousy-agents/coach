package baseline_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/baseline"
	"github.com/lousy-agents/coach/internal/coachapi/store/memory"
)

// memoryFencedWriter is a leaseWriter-equivalent over MemoryStore so acceptance
// exercises the real fenced InsertFindings/InsertDiagnostics path.
type memoryFencedWriter struct {
	store *memory.Store
	lease coachapi.ClaimLease
}

var _ baseline.JobWriter = (*memoryFencedWriter)(nil)

func newMemoryFencedWriter(job coachapi.Job) (*memory.Store, *memoryFencedWriter) {
	GinkgoHelper()
	store := memory.NewStore()
	queued := job
	queued.Status = coachapi.JobStatusQueued
	Expect(store.CreateJob(context.Background(), queued)).To(Succeed())
	lease, err := store.ClaimJob(context.Background(), job.ID, "baseline-test-worker", time.Now().UTC(), time.Minute)
	Expect(err).NotTo(HaveOccurred())
	return store, &memoryFencedWriter{store: store, lease: lease}
}

func (w *memoryFencedWriter) Lease() coachapi.ClaimLease { return w.lease }

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

func (w *memoryFencedWriter) InsertDiagnostics(ctx context.Context, diagnostics []coachapi.JobDiagnostic) error {
	cap := &captureWriter{lease: w.lease}
	if err := cap.InsertDiagnostics(ctx, diagnostics); err != nil {
		return err
	}
	stamped := append([]coachapi.JobDiagnostic(nil), diagnostics...)
	for i := range stamped {
		stamped[i].JobID = w.lease.JobID
		stamped[i].Attempt = w.lease.Attempt
	}
	return w.store.InsertDiagnostics(ctx, w.lease.JobID, w.lease.WorkerID, w.lease.Attempt, stamped)
}
