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
				"fixture-acme-widgets",
				"repo-read-happy-path",
				"GET",
				"/repos/acme/widgets/contents/hello.txt",
				acceptanceharness.AuthModeInstallation,
			)

			Expect(rec.SchemaVersion).To(Equal(acceptanceharness.FixtureSchemaVersion))
			Expect(rec.FixtureID).To(Equal("fixture-acme-widgets"))
			Expect(rec.Scenario).To(Equal("repo-read-happy-path"))
			Expect(rec.Method).To(Equal("GET"))
			Expect(rec.Path).To(Equal("/repos/acme/widgets/contents/hello.txt"))
			Expect(rec.AuthMode).To(Equal(acceptanceharness.AuthModeInstallation))
		})
	})

	Context("schema-stability contract", func() {
		It("marshals a RequestRecord built via NewRequestRecord to the exact expected JSON shape", func() {
			rec := acceptanceharness.NewRequestRecord(
				"fixture-acme-widgets",
				"repo-read-happy-path",
				"GET",
				"/repos/acme/widgets/contents/hello.txt",
				acceptanceharness.AuthModeInstallation,
			)

			got, err := json.Marshal(rec)
			Expect(err).NotTo(HaveOccurred())

			Expect(got).To(MatchJSON(`{
				"schema_version": 1,
				"fixture_id": "fixture-acme-widgets",
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

	Context("when two fixtures share the same scenario name", func() {
		// PR #80 review, second round: RequestRecord previously retained
		// only Scenario, so two fixture files sharing a scenario name (e.g.
		// both defining a "not-found" behavior) could not be told apart in
		// a recorded request. FixtureID is what keeps them distinguishable.
		It("remains distinguishable via FixtureID alone even though Scenario is identical", func() {
			fromFixtureA := acceptanceharness.NewRequestRecord("fixture-a", "not-found", "GET", "/x", acceptanceharness.AuthModeInstallation)
			fromFixtureB := acceptanceharness.NewRequestRecord("fixture-b", "not-found", "GET", "/x", acceptanceharness.AuthModeInstallation)

			Expect(fromFixtureA.Scenario).To(Equal(fromFixtureB.Scenario))
			Expect(fromFixtureA.FixtureID).NotTo(Equal(fromFixtureB.FixtureID))
			Expect(fromFixtureA.FixtureID).To(Equal("fixture-a"))
			Expect(fromFixtureB.FixtureID).To(Equal("fixture-b"))
		})
	})

	Context("when NewFixtureHeader builds a FixtureHeader", func() {
		It("stamps SchemaVersion with the current FixtureSchemaVersion and the given fixture ID", func() {
			header := acceptanceharness.NewFixtureHeader("fixture-acme-widgets")

			Expect(header.SchemaVersion).To(Equal(acceptanceharness.FixtureSchemaVersion))
			Expect(header.FixtureID).To(Equal("fixture-acme-widgets"))
		})
	})

	Context("when Recorder.Record is called for several requests", func() {
		It("Records returns them in insertion order and as a defensive copy", func() {
			var recorder acceptanceharness.Recorder

			first := acceptanceharness.NewRequestRecord("fixture-a", "scenario-a", "GET", "/one", acceptanceharness.AuthModeNone)
			second := acceptanceharness.NewRequestRecord("fixture-a", "scenario-b", "POST", "/two", acceptanceharness.AuthModeOAuth)
			third := acceptanceharness.NewRequestRecord("fixture-a", "scenario-c", "GET", "/three", acceptanceharness.AuthModeRejected)

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
							"fixture-concurrent",
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
