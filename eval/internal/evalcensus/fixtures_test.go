package evalcensus_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

const fixtureSHA = "0123456789abcdef0123456789abcdef01234567"

func strPtr(s string) *string { return &s }

// automatedRecord is a valid detector-agreement record with the given id and
// value, which callers vary one field at a time.
func automatedRecord(id string, value string) evalcensus.Record {
	return evalcensus.Record{
		RecordID: id,
		Target: evalcensus.Target{
			CorpusID: "pilot", SHA: fixtureSHA, RuleID: "complexity.cognitive_complexity",
			Path: "pkg/a/a.go", Lines: [2]int{3, 9},
		},
		Measure: "detector_agreement",
		Value:   json.RawMessage(value),
		Source:  evalcensus.Source{Kind: "detector", Ref: "gocognit v1.3.0"},
		Rater:   evalcensus.Rater{Kind: "automated"},
		Date:    "2026-11-26",
	}
}

// humanRecord is a valid human verdict with the given id and value.
func humanRecord(id string, value string) evalcensus.Record {
	record := automatedRecord(id, value)
	record.Measure = "human_verdict"
	record.Source = evalcensus.Source{Kind: "human", Ref: "review"}
	record.Rater = evalcensus.Rater{ID: strPtr("a reviewer"), Kind: "human"}
	record.Rationale = "read the function"
	return record
}

func ledgerPath() string {
	return filepath.Join(GinkgoT().TempDir(), "pilot", "records.jsonl")
}

func readFile(path string) string {
	raw, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	return string(raw)
}

func writeJSON(value any) string {
	raw, err := json.Marshal(value)
	Expect(err).NotTo(HaveOccurred())
	path := filepath.Join(GinkgoT().TempDir(), "corpus.json")
	Expect(os.WriteFile(path, raw, 0o644)).To(Succeed())
	return path
}

type signal struct {
	RuleID   string `json:"rule_id"`
	Evidence string `json:"evidence,omitempty"`
}

// coachReport is a Coach --format json report holding the given signals.
func coachReport(signals ...signal) []byte {
	if signals == nil {
		signals = []signal{}
	}
	raw, err := json.Marshal(map[string]any{"schema_version": "1", "signals": signals})
	Expect(err).NotTo(HaveOccurred())
	return raw
}

func repeated(n int, s signal) []signal {
	signals := make([]signal, n)
	for i := range signals {
		signals[i] = s
	}
	return signals
}
