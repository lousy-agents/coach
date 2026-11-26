package main_test

import (
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
	"github.com/lousy-agents/coach/eval/internal/evalharness"
	"github.com/lousy-agents/coach/eval/internal/evalhistory"
)

var _ = Describe("the instant a ledger line records", func() {
	instant := time.Date(2001, 2, 3, 23, 59, 59, 999, time.UTC)
	coach := evalcensus.CoachIdentity{Path: "/tmp/build/coach", SHA256: "abababababababababababababababababababababababababababababababab"}

	DescribeTable("takes its recorded date and run id from the run's one instant",
		func(build func() ([]byte, error), prefix string) {
			data, err := build()
			Expect(err).NotTo(HaveOccurred())
			var line map[string]any
			Expect(json.Unmarshal(data, &line)).To(Succeed())
			Expect(line["recorded"]).To(Equal("2001-02-03"))
			Expect(line["run"].(map[string]any)["run_id"]).To(Equal(prefix + "-20010203T235959.000000999Z"))
		},
		Entry("a census line", func() ([]byte, error) {
			return evalharness.CensusLine(evalcensus.Summary{CorpusID: "pilot", Coach: coach, Repositories: []evalcensus.RepositoryResult{}}, 0, instant)
		}, "census"),
		Entry("a history line", func() ([]byte, error) {
			return evalharness.HistoryLine(evalhistory.Summary{
				CorpusID: "pilot", Coach: coach, Seed: 7, ControlsPerFlagged: 3, FixPattern: "fix",
				Statement: evalhistory.Statement, Repositories: []evalhistory.RepositoryHistory{},
			}, 0, instant)
		}, "history"),
	)
})
