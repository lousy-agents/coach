package evalhistory_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
	"github.com/lousy-agents/coach/eval/internal/evalhistory"
)

var _ = Describe("the instant a history run is recorded at", func() {
	It("dates the records and the ledger line from one reading of a clock that ticks past midnight on the next", func() {
		h := newHistory()
		since := h.commit("start", map[string]string{"p/s.go": unitsSource})
		until := h.commit("fix: B", map[string]string{"p/s.go": replaceLine(unitsSource, 8, "\tx := 20")})
		raw, err := json.Marshal(corpusFor(h, since, until, goRule(cognitive)))
		Expect(err).NotTo(HaveOccurred())
		corpus := filepath.Join(GinkgoT().TempDir(), "corpus.json")
		Expect(os.WriteFile(corpus, raw, 0o644)).To(Succeed())
		root := GinkgoT().TempDir()
		instant := time.Date(2001, 2, 3, 23, 59, 59, 999, time.UTC)
		readings := 0
		clock := func() time.Time {
			readings++
			return instant.Add(time.Duration(readings-1) * time.Second)
		}
		var recorded []time.Time
		line := func(_ evalhistory.Summary, _ int, at time.Time) ([]byte, error) {
			recorded = append(recorded, at)
			return []byte("{}\n"), nil
		}
		var stdout, stderr bytes.Buffer
		coach := fakeCoach([]reportSignal{functionSignal(cognitive, "p/s.go", "B", 6, 9)})

		exit := evalhistory.History([]string{"--coach", coach, "--census-root", root, "--ledger", filepath.Join(root, "runs.jsonl"), "--seed", "7", corpus}, &stdout, &stderr, line, clock)

		Expect(exit).To(Equal(0), stderr.String())
		Expect(readings).To(Equal(1))
		Expect(recorded).To(Equal([]time.Time{instant}))
		records, err := evalcensus.LoadRecords(filepath.Join(root, "history", "records.jsonl"))
		Expect(err).NotTo(HaveOccurred())
		Expect(records).To(HaveLen(1))
		Expect(records[0].Date).To(Equal("2001-02-03"))
	})
})
