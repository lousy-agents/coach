package evalcensus_test

import (
	"encoding/json"
	"fmt"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

const scopedRule = "complexity.cognitive_complexity"

// finding is one Coach signal in a report, at path, with a fingerprint the sample keys on.
func finding(path, fingerprint string) map[string]any {
	return map[string]any{
		"rule_id": scopedRule, "path": path, "subject": "f", "fingerprint": fingerprint, "evidence": "e",
	}
}

func findingsReport(signals ...map[string]any) []byte {
	raw, err := json.Marshal(map[string]any{"signals": signals})
	Expect(err).NotTo(HaveOccurred())
	return raw
}

// manyFindings is count findings at src/f<i>.go, each with a fingerprint fp<i>.
func manyFindings(count int) []map[string]any {
	signals := make([]map[string]any, 0, count)
	for i := range count {
		signals = append(signals, finding(fmt.Sprintf("src/f%02d.go", i), fmt.Sprintf("fp%02d", i)))
	}
	return signals
}

// statusesOf runs the census's status step over the first repository, with the patch applied to its manifest
// entry, and returns each rule's status as JSON.
func statusesOf(patch map[string]any, report []byte) []map[string]any {
	manifest := scopedCorpus()
	for key, value := range patch {
		firstRepository(manifest)[key] = value
	}
	corpus, err := evalcensus.LoadCorpus(writeJSON(manifest))
	Expect(err).NotTo(HaveOccurred())
	raw, err := json.Marshal(evalcensus.StatusesFromReport(corpus, corpus.Repositories[0], report))
	Expect(err).NotTo(HaveOccurred())
	var statuses []map[string]any
	Expect(json.Unmarshal(raw, &statuses)).To(Succeed())
	return statuses
}

func statusFor(statuses []map[string]any, ruleID string) map[string]any {
	for _, status := range statuses {
		if status["rule_id"] == ruleID {
			return status
		}
	}
	Fail("no status for rule " + ruleID)
	return nil
}

var _ = Describe("the census's path scope and sample", func() {
	When("the manifest names an include glob", func() {
		It("counts only the findings whose path the glob admits", func() {
			statuses := statusesOf(map[string]any{"include": []any{"src/**"}}, findingsReport(
				finding("src/a.go", "fp1"),
				finding("lib/b.go", "fp2"),
			))

			Expect(statusFor(statuses, scopedRule)["findings"]).To(BeEquivalentTo(1))
		})
	})

	When("the manifest names an exclude glob", func() {
		It("removes the findings whose path it matches", func() {
			statuses := statusesOf(map[string]any{"exclude": []any{"src/gen/**"}}, findingsReport(
				finding("src/gen/x.go", "fp1"),
				finding("src/a.go", "fp2"),
			))

			Expect(statusFor(statuses, scopedRule)["findings"]).To(BeEquivalentTo(1))
		})
	})

	When("the manifest names no include glob", func() {
		It("counts every path the fixed exclusions do not remove", func() {
			statuses := statusesOf(nil, findingsReport(
				finding("src/a.go", "fp1"),
				finding("lib/b.go", "fp2"),
			))

			Expect(statusFor(statuses, scopedRule)["findings"]).To(BeEquivalentTo(2))
		})
	})

	When("a finding lies under eval/ at the repository root", func() {
		It("excludes it, and counts an eval/ directory below the root", func() {
			statuses := statusesOf(nil, findingsReport(
				finding("eval/x.go", "fp1"),
				finding("pkg/eval/y.go", "fp2"),
			))

			Expect(statusFor(statuses, scopedRule)["findings"]).To(BeEquivalentTo(1))
		})
	})

	When("a finding lies under a vendor/ directory at any depth", func() {
		It("excludes it", func() {
			statuses := statusesOf(nil, findingsReport(
				finding("vendor/v.go", "fp1"),
				finding("pkg/vendor/w.go", "fp2"),
				finding("src/a.go", "fp3"),
			))

			Expect(statusFor(statuses, scopedRule)["findings"]).To(BeEquivalentTo(1))
		})
	})

	When("a glob uses **, * or ?", func() {
		It("lets ** cross segments", func() {
			statuses := statusesOf(map[string]any{"include": []any{"src/**/a.go"}}, findingsReport(
				finding("src/x/y/a.go", "fp1"),
				finding("src/b.go", "fp2"),
			))

			Expect(statusFor(statuses, scopedRule)["findings"]).To(BeEquivalentTo(1))
		})

		It("keeps * inside one segment", func() {
			statuses := statusesOf(map[string]any{"include": []any{"src/*.go"}}, findingsReport(
				finding("src/a.go", "fp1"),
				finding("src/x/b.go", "fp2"),
			))

			Expect(statusFor(statuses, scopedRule)["findings"]).To(BeEquivalentTo(1))
		})

		It("matches ? to exactly one character", func() {
			statuses := statusesOf(map[string]any{"include": []any{"src/?.go"}}, findingsReport(
				finding("src/a.go", "fp1"),
				finding("src/ab.go", "fp2"),
			))

			Expect(statusFor(statuses, scopedRule)["findings"]).To(BeEquivalentTo(1))
		})
	})

	When("a rule has at most 30 findings", func() {
		It("keeps all of them, under the frame all_findings", func() {
			statuses := statusesOf(nil, findingsReport(manyFindings(30)...))

			status := statusFor(statuses, scopedRule)
			Expect(status["frame"]).To(Equal("all_findings"))
			Expect(status["sampled_keys"]).To(HaveLen(30))
		})
	})

	When("a rule has more than 30 findings", func() {
		It("draws 30 of them, under the frame sampled", func() {
			statuses := statusesOf(nil, findingsReport(manyFindings(31)...))

			status := statusFor(statuses, scopedRule)
			Expect(status["frame"]).To(Equal("sampled"))
			Expect(status["sampled_keys"]).To(HaveLen(30))
			Expect(status["findings"]).To(BeEquivalentTo(31))
		})
	})

	When("the manifest names a sample size", func() {
		It("draws that many findings instead of 30", func() {
			statuses := statusesOf(map[string]any{"sample": map[string]any{"seed": 7, "size": 5}},
				findingsReport(manyFindings(6)...))

			status := statusFor(statuses, scopedRule)
			Expect(status["frame"]).To(Equal("sampled"))
			Expect(status["sampled_keys"]).To(HaveLen(5))
		})
	})

	When("the same seed meets the same findings in another input order", func() {
		It("draws the same findings", func() {
			signals := manyFindings(40)
			first := statusFor(statusesOf(nil, findingsReport(signals...)), scopedRule)

			reversed := slices.Clone(signals)
			slices.Reverse(reversed)
			second := statusFor(statusesOf(nil, findingsReport(reversed...)), scopedRule)

			Expect(second["sampled_keys"]).To(Equal(first["sampled_keys"]))
		})
	})

	When("a different seed is named", func() {
		It("draws different findings", func() {
			signals := findingsReport(manyFindings(40)...)
			seven := statusFor(statusesOf(nil, signals), scopedRule)
			eight := statusFor(statusesOf(map[string]any{"sample": map[string]any{"seed": 8}}, signals), scopedRule)

			Expect(eight["sampled_keys"]).NotTo(Equal(seven["sampled_keys"]))
		})
	})

	When("a repository's sample is recorded", func() {
		It("gives the seed and the size in the repository's summary JSON", func() {
			raw := fmt.Sprintf(`{"id":"coach","sha":%q,"coach_exit":0,"sample":{"seed":7,"size":30},"statuses":[]}`, fixtureSHA)
			var result evalcensus.RepositoryResult
			Expect(json.Unmarshal([]byte(raw), &result)).To(Succeed())

			out, err := json.Marshal(result)

			Expect(err).NotTo(HaveOccurred())
			Expect(string(out)).To(ContainSubstring(`"sample":{"seed":7,"size":30}`))
		})
	})

	When("an analysis_status record is written", func() {
		It("gives the total and the sampled counts in its rationale", func() {
			statuses := statusesOf(nil, findingsReport(manyFindings(31)...))
			raw, err := json.Marshal(statuses)
			Expect(err).NotTo(HaveOccurred())
			var typed []evalcensus.RuleStatus
			Expect(json.Unmarshal(raw, &typed)).To(Succeed())
			summary := evalcensus.Summary{
				CorpusID:     "pilot",
				Repositories: []evalcensus.RepositoryResult{{ID: "coach", SHA: fixtureSHA, Statuses: typed}},
			}

			records := evalcensus.StatusRecords(summary, "2026-11-26")

			var rationale string
			for _, record := range records {
				if record.Target.RuleID == scopedRule {
					rationale = record.Rationale
				}
			}
			Expect(rationale).To(ContainSubstring("findings=31"))
			Expect(rationale).To(ContainSubstring("sampled=30"))
		})
	})
})
