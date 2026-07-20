package acceptanceharness_test

import (
	"encoding/json"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

var _ = Describe("shared fixture/recording contract", func() {
	Context("when NewRequestRecord builds a RequestRecord", func() {
		It("stamps SchemaVersion with the current FixtureSchemaVersion and sets the given fields", func() {
			rec := acceptanceharness.NewRequestRecord(
				"repo-read-happy-path",
				"GET",
				"/repos/acme/widgets/contents/hello.txt",
				acceptanceharness.AuthModeInstallation,
			)

			Expect(rec.SchemaVersion).To(Equal(acceptanceharness.FixtureSchemaVersion))
			Expect(rec.Scenario).To(Equal("repo-read-happy-path"))
			Expect(rec.Method).To(Equal("GET"))
			Expect(rec.Path).To(Equal("/repos/acme/widgets/contents/hello.txt"))
			Expect(rec.AuthMode).To(Equal(acceptanceharness.AuthModeInstallation))
		})
	})

	Context("schema-stability contract", func() {
		It("marshals a RequestRecord built via NewRequestRecord to the exact expected JSON shape", func() {
			rec := acceptanceharness.NewRequestRecord(
				"repo-read-happy-path",
				"GET",
				"/repos/acme/widgets/contents/hello.txt",
				acceptanceharness.AuthModeInstallation,
			)

			got, err := json.Marshal(rec)
			Expect(err).NotTo(HaveOccurred())

			Expect(got).To(MatchJSON(`{
				"schema_version": 1,
				"scenario": "repo-read-happy-path",
				"method": "GET",
				"path": "/repos/acme/widgets/contents/hello.txt",
				"auth_mode": "installation"
			}`))
		})
	})

	Context("AuthMode vocabulary", func() {
		It("has exactly the documented string values", func() {
			Expect(string(acceptanceharness.AuthModeOAuth)).To(Equal("oauth"))
			Expect(string(acceptanceharness.AuthModeInstallation)).To(Equal("installation"))
			Expect(string(acceptanceharness.AuthModeNone)).To(Equal("none"))
			Expect(string(acceptanceharness.AuthModeRejected)).To(Equal("rejected"))
		})
	})

	Context("when Recorder.Record is called for several requests", func() {
		It("Records returns them in insertion order and as a defensive copy", func() {
			var recorder acceptanceharness.Recorder

			first := acceptanceharness.NewRequestRecord("scenario-a", "GET", "/one", acceptanceharness.AuthModeNone)
			second := acceptanceharness.NewRequestRecord("scenario-b", "POST", "/two", acceptanceharness.AuthModeOAuth)
			third := acceptanceharness.NewRequestRecord("scenario-c", "GET", "/three", acceptanceharness.AuthModeRejected)

			recorder.Record(first)
			recorder.Record(second)
			recorder.Record(third)

			got := recorder.Records()
			Expect(got).To(Equal([]acceptanceharness.RequestRecord{first, second, third}))

			// Mutating the returned slice must not affect the Recorder's
			// internal state on a subsequent Records() call.
			got[0].Scenario = "mutated"

			gotAgain := recorder.Records()
			Expect(gotAgain).To(Equal([]acceptanceharness.RequestRecord{first, second, third}))
		})
	})

	Context("when multiple goroutines call Record concurrently", func() {
		It("is safe under -race and every record is preserved (mirrors clock_acceptance_test.go's concurrency-proof pattern)", func() {
			var recorder acceptanceharness.Recorder

			const workers = 8
			const iterationsPerWorker = 50
			const total = workers * iterationsPerWorker

			var wg sync.WaitGroup
			wg.Add(workers)
			for i := 0; i < workers; i++ {
				go func(worker int) {
					defer wg.Done()
					for j := 0; j < iterationsPerWorker; j++ {
						recorder.Record(acceptanceharness.NewRequestRecord(
							"concurrent-scenario",
							"GET",
							"/concurrent",
							acceptanceharness.AuthModeNone,
						))
					}
				}(i)
			}
			wg.Wait()

			Expect(recorder.Records()).To(HaveLen(total))
		})
	})
})
