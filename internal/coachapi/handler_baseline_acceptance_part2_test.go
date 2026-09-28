package coachapi_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"runtime"

	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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

func (f *fakeTreeSource) ResolveCommitSHA(_ context.Context, _, _, ref string) (string, error) {
	f.resolveCalls++
	f.lastResolveRef = ref
	if f.resolveErr != nil {
		return "", f.resolveErr
	}
	if f.resolvedSHA != "" {
		return f.resolvedSHA, nil
	}
	if ref == "" {
		return "HEAD", nil
	}
	return ref, nil
}

func (w *captureWriter) Lease() coachapi.ClaimLease { return w.lease }

func (w *memoryFencedWriter) Lease() coachapi.ClaimLease { return w.lease }

func newMemoryFencedWriter(job coachapi.Job) (*coachapi.MemoryStore, *memoryFencedWriter) {
	GinkgoHelper()
	store := coachapi.NewMemoryStore()
	queued := job
	queued.Status = coachapi.JobStatusQueued
	Expect(store.CreateJob(context.Background(), queued)).To(Succeed())
	lease, err := store.ClaimJob(context.Background(), job.ID, "baseline-test-worker", time.Now().UTC(), time.Minute)
	Expect(err).NotTo(HaveOccurred())
	return store, &memoryFencedWriter{store: store, lease: lease}
}

func baselineFixtureRoot() string {
	GinkgoHelper()
	_, thisFile, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue())
	src := filepath.Join(filepath.Dir(thisFile), "testdata", "baseline_fixture")
	_, err := os.Stat(src)
	Expect(err).NotTo(HaveOccurred(), "baseline fixture root must exist at %s", src)
	// The committed tree returns new values. Handler specs still need two
	// hidden-input signals, so the scan root is a copy with those writes
	// overlaid. The overlay is not part of the repository scan corpus.
	root := GinkgoT().TempDir()
	Expect(os.CopyFS(root, os.DirFS(src))).To(Succeed())
	Expect(os.WriteFile(filepath.Join(root, "widget", "update_test.go"), []byte(baselineMutatingUpdateGo), 0o644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(root, "widget", "reset_test.go"), []byte(baselineMutatingResetGo), 0o644)).To(Succeed())
	return root
}

func baselineJob(params coachapi.RepoBaselineScanParams) coachapi.Job {
	GinkgoHelper()
	raw, err := json.Marshal(params)
	Expect(err).NotTo(HaveOccurred())
	return coachapi.Job{
		ID:                "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		Kind:              coachapi.JobKindRepoBaselineScan,
		Params:            raw,
		Status:            coachapi.JobStatusRunning,
		Attempt:           1,
		CreatedAt:         time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC),
		CreatedByProvider: "github",
		CreatedBySubject:  "1",
		CreatedByLogin:    "octocat",
	}
}

func baselineRSAKey() []byte {
	GinkgoHelper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	return pem.EncodeToMemory(block)
}
