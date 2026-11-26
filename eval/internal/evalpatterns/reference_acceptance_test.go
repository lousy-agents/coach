package evalpatterns_test

import (
	"errors"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalpatterns"
)

func validReference() evalpatterns.Reference {
	return evalpatterns.Reference{Label: "stand-in", SHA: sampleSHA, Language: "go", Licence: "MIT"}
}

var _ = Describe("a reference manifest entry", func() {
	When("the entry has a licence, a 40-hex SHA and the Go language", func() {
		It("is accepted", func() {
			Expect(validReference().Check()).To(Succeed())
		})
	})

	DescribeTable("the pattern extractor refuses the entry with exit 2, naming its label",
		func(change func(*evalpatterns.Reference), reason string) {
			reference := validReference()
			change(&reference)

			err := reference.Check()

			var refusal evalpatterns.Refusal
			Expect(errors.As(err, &refusal)).To(BeTrue(), "Check returned %v", err)
			Expect(refusal.Exit).To(Equal(2))
			Expect(refusal.Label).To(Equal("stand-in"))
			Expect(err.Error()).To(Equal(`reference "stand-in": ` + reason))
		},
		Entry("if its licence is missing (AC-1.5)",
			func(r *evalpatterns.Reference) { r.Licence = "" }, "licence is required"),
		Entry("if its licence is only whitespace once trimmed (AC-1.5)",
			func(r *evalpatterns.Reference) { r.Licence = " \t\n" }, "licence is required"),
		Entry("if its SHA is abbreviated (AC-1.6)",
			func(r *evalpatterns.Reference) { r.SHA = sampleSHA[:12] }, `sha "0123456789ab" is not a 40-hex commit SHA`),
		Entry("if its SHA is one character too long (AC-1.6)",
			func(r *evalpatterns.Reference) { r.SHA = sampleSHA + "8" }, `sha "`+sampleSHA+`8" is not a 40-hex commit SHA`),
		Entry("if its SHA is upper-case hex (AC-1.6)",
			func(r *evalpatterns.Reference) { r.SHA = "0123456789ABCDEF0123456789ABCDEF01234567" },
			`sha "0123456789ABCDEF0123456789ABCDEF01234567" is not a 40-hex commit SHA`),
		Entry("if its SHA is a branch name (AC-1.6)",
			func(r *evalpatterns.Reference) { r.SHA = "main" }, `sha "main" is not a 40-hex commit SHA`),
		Entry("if its language is TypeScript (AC-1.7)",
			func(r *evalpatterns.Reference) { r.Language = "typescript" }, `language "typescript" is not go`),
		Entry("if its language is missing (AC-1.7)",
			func(r *evalpatterns.Reference) { r.Language = "" }, `language "" is not go`),
		Entry("if its language is spelled with a capital (AC-1.7)",
			func(r *evalpatterns.Reference) { r.Language = "Go" }, `language "Go" is not go`),
	)
})

var _ = Describe("loading a reference manifest", func() {
	When("the manifest lists entries", func() {
		It("reads each entry's label, SHA, language and licence, including one it will refuse", func() {
			path := writeManifest(map[string]any{"schema_version": "1", "references": []any{
				map[string]any{"label": "stand-in", "sha": sampleSHA, "language": "go", "licence": "MIT"},
				map[string]any{"label": "unlicensed", "sha": sampleSHA, "language": "go"},
			}})

			manifest, err := evalpatterns.LoadManifest(path)

			Expect(err).NotTo(HaveOccurred())
			Expect(manifest.References).To(Equal([]evalpatterns.Reference{
				validReference(),
				{Label: "unlicensed", SHA: sampleSHA, Language: "go"},
			}))
		})
	})

	DescribeTable("the manifest as a whole is unusable input",
		func(content any, message string) {
			_, err := evalpatterns.LoadManifest(writeManifest(content))

			Expect(err).To(MatchError(ContainSubstring(message)))
		},
		Entry("when an entry carries a field outside label, sha, language and licence, such as the clone path",
			map[string]any{"schema_version": "1", "references": []any{
				map[string]any{"label": "stand-in", "sha": sampleSHA, "language": "go", "licence": "MIT", "path": "/src/x"},
			}}, `unknown field "path"`),
		Entry("when its schema_version is not 1",
			map[string]any{"schema_version": "2", "references": []any{
				map[string]any{"label": "stand-in", "sha": sampleSHA, "language": "go", "licence": "MIT"},
			}}, `schema_version must be "1"`),
		Entry("when it lists no entries",
			map[string]any{"schema_version": "1", "references": []any{}}, "references is empty"),
		Entry("when an entry's label is not a lowercase path segment",
			map[string]any{"schema_version": "1", "references": []any{
				map[string]any{"label": "../escape", "sha": sampleSHA, "language": "go", "licence": "MIT"},
			}}, `label "../escape" must be a lowercase path segment`),
		Entry("when an entry's label is longer than the profile schema allows",
			map[string]any{"schema_version": "1", "references": []any{
				map[string]any{"label": strings.Repeat("a", 65), "sha": sampleSHA, "language": "go", "licence": "MIT"},
			}}, `is longer than 64 characters`),
		Entry("when two entries share a label",
			map[string]any{"schema_version": "1", "references": []any{
				map[string]any{"label": "stand-in", "sha": sampleSHA, "language": "go", "licence": "MIT"},
				map[string]any{"label": "stand-in", "sha": sampleSHA, "language": "go", "licence": "MIT"},
			}}, `label "stand-in" appears twice`),
	)
})
