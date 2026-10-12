package baseline_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
)

func baselineFixtureRoot() string {
	GinkgoHelper()
	_, thisFile, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue())
	root := filepath.Join(filepath.Dir(thisFile), "..", "testdata", "baseline_fixture")
	_, err := os.Stat(root)
	Expect(err).NotTo(HaveOccurred(), "baseline fixture root must exist at %s", root)
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

func handlerSourcedNames(calls []agentloop.RecordedCall) []string {
	var names []string
	for _, c := range calls {
		if c.Source == agentloop.CallSourceHandler {
			names = append(names, c.Name)
		}
	}
	return names
}

func findingUniqKey(f coachapi.JobFinding) string {
	rubric := ""
	if f.RubricID != nil {
		rubric = *f.RubricID
	}
	// JobID/Attempt stamped by leaseWriter in production; capture keys source+rubric+hash.
	return string(f.Source) + "\x00" + rubric + "\x00" + f.PayloadHash
}
