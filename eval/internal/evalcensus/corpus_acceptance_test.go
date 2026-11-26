package evalcensus_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

func validCorpus() map[string]any {
	return map[string]any{
		"corpus_id": "pilot",
		"repositories": []any{
			map[string]any{"id": "coach", "path": "/repos/coach", "sha": fixtureSHA, "language": "go", "project_config": ".lousy-agents/coach.json"},
			map[string]any{"id": "iam", "path": "/repos/iam", "sha": fixtureSHA, "language": "typescript"},
		},
		"rules": []any{
			map[string]any{"rule_id": "complexity.cognitive_complexity", "languages": []any{"go", "typescript"}},
			map[string]any{"rule_id": "coupling.deep_relative_import", "languages": []any{"typescript"}},
		},
	}
}

var _ = Describe("the corpus manifest", func() {
	When("it is well formed", func() {
		It("loads its repositories and rules", func() {
			corpus, err := evalcensus.LoadCorpus(writeJSON(validCorpus()))
			Expect(err).NotTo(HaveOccurred())
			Expect(corpus.ID).To(Equal("pilot"))
			Expect(corpus.Repositories).To(HaveLen(2))
			Expect(corpus.Repositories[0].ProjectConfig).To(Equal(".lousy-agents/coach.json"))
			Expect(corpus.Rules[1].Languages).To(Equal([]string{"typescript"}))
		})
	})

	DescribeTable("a malformed manifest is refused, naming the problem",
		func(mutate func(map[string]any), problem string) {
			manifest := validCorpus()
			mutate(manifest)

			_, err := evalcensus.LoadCorpus(writeJSON(manifest))
			Expect(err).To(MatchError(ContainSubstring(problem)))
		},
		Entry("no corpus_id", func(m map[string]any) { m["corpus_id"] = "" }, "corpus_id"),
		Entry("a corpus_id that is not a path segment", func(m map[string]any) { m["corpus_id"] = "../up" }, "corpus_id"),
		Entry("a corpus_id of .", func(m map[string]any) { m["corpus_id"] = "." }, "corpus_id"),
		Entry("a corpus_id of ..", func(m map[string]any) { m["corpus_id"] = ".." }, "corpus_id"),
		Entry("a corpus_id starting with a dot", func(m map[string]any) { m["corpus_id"] = ".hidden" }, "corpus_id"),
		Entry("a corpus_id holding a slash", func(m map[string]any) { m["corpus_id"] = "a/b" }, "corpus_id"),
		Entry("an uppercase corpus_id", func(m map[string]any) { m["corpus_id"] = "Pilot" }, "corpus_id"),
		Entry("a corpus_id with trailing junk", func(m map[string]any) { m["corpus_id"] = "pilot!" }, "corpus_id"),
		Entry("a misspelt field", func(m map[string]any) {
			m["repositories"].([]any)[0].(map[string]any)["projectconfig"] = "x.json"
		}, "projectconfig"),
		Entry("an uppercase SHA", func(m map[string]any) {
			m["repositories"].([]any)[0].(map[string]any)["sha"] = strings.ToUpper(fixtureSHA)
		}, "sha"),
		Entry("a SHA with trailing junk", func(m map[string]any) {
			m["repositories"].([]any)[0].(map[string]any)["sha"] = fixtureSHA + "0"
		}, "sha"),
		Entry("no repositories", func(m map[string]any) { m["repositories"] = []any{} }, "repositories"),
		Entry("no rules", func(m map[string]any) { m["rules"] = []any{} }, "rules"),
		Entry("a floating ref in place of a SHA", func(m map[string]any) {
			m["repositories"].([]any)[0].(map[string]any)["sha"] = "main"
		}, "sha"),
		Entry("an unknown language", func(m map[string]any) {
			m["repositories"].([]any)[0].(map[string]any)["language"] = "rust"
		}, "language"),
		Entry("a repository with no id", func(m map[string]any) {
			m["repositories"].([]any)[0].(map[string]any)["id"] = ""
		}, "id is empty"),
		Entry("a rule with no rule_id", func(m map[string]any) {
			m["rules"].([]any)[0].(map[string]any)["rule_id"] = ""
		}, "rule_id is empty"),
		Entry("a repository with no path", func(m map[string]any) {
			m["repositories"].([]any)[0].(map[string]any)["path"] = ""
		}, "path"),
		Entry("two repositories with one id", func(m map[string]any) {
			m["repositories"].([]any)[1].(map[string]any)["id"] = "coach"
		}, "duplicate repository"),
		Entry("a rule listed twice", func(m map[string]any) {
			m["rules"].([]any)[1].(map[string]any)["rule_id"] = "complexity.cognitive_complexity"
		}, "duplicate rule"),
		Entry("a rule with no languages", func(m map[string]any) {
			m["rules"].([]any)[0].(map[string]any)["languages"] = []any{}
		}, "languages"),
		Entry("a rule with an unknown language", func(m map[string]any) {
			m["rules"].([]any)[0].(map[string]any)["languages"] = []any{"go", "cobol"}
		}, "language"),
	)

	When("the file is not JSON", func() {
		It("refuses it", func() {
			_, err := evalcensus.LoadCorpus(writeJSON("not an object"))
			Expect(err).To(HaveOccurred())
		})
	})
})
