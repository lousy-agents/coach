package evalpatterns_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalpatterns"
)

func nested(profile map[string]any, key string) map[string]any {
	return profile[key].(map[string]any)
}

var _ = Describe("the profile schema (AC-1.4)", func() {
	When("a profile holds only derived statistics, the label, the SHA and the licence", func() {
		It("validates", func() {
			Expect(evalpatterns.ValidateProfile(profileJSON(validProfile()))).To(Succeed())
		})
	})

	When("a dimension has no observation", func() {
		It("validates with an empty distribution", func() {
			profile := validProfile()
			nested(profile, "dimensions")["depth"] = map[string]any{}
			profile["roles"] = map[string]any{}
			profile["not_covered"] = []any{}

			Expect(evalpatterns.ValidateProfile(profileJSON(profile))).To(Succeed())
		})
	})

	DescribeTable("a profile fails validation",
		func(change func(map[string]any)) {
			profile := validProfile()
			change(profile)

			Expect(evalpatterns.ValidateProfile(profileJSON(profile))).NotTo(Succeed())
		},
		Entry("when it carries a top-level field outside the schema, such as a repository name",
			func(p map[string]any) { p["repository"] = "acme/widgets" }),
		Entry("when a role carries a field outside the schema, such as a package path",
			func(p map[string]any) { nested(p, "roles")["api"].(map[string]any)["path"] = "internal/api" }),
		Entry("when a role is named by the repository's layer name rather than a generator role",
			func(p map[string]any) {
				nested(p, "roles")["handlers"] = map[string]any{"packages": map[string]any{"1": 1}}
			}),
		Entry("when a distribution bucket is a name rather than a count",
			func(p map[string]any) { nested(nested(p, "shapes"), "nesting")["ParseConfig"] = 1 }),
		Entry("when a distribution value is negative",
			func(p map[string]any) { nested(nested(p, "shapes"), "nesting")["1"] = -1 }),
		Entry("when the entry distribution carries a kind outside the three signatures",
			func(p map[string]any) { nested(nested(p, "dimensions"), "entry")["command"] = 1 }),
		Entry("when trap kind is set (AC-2.7)",
			func(p map[string]any) { nested(p, "dimensions")["trap"] = "layering" }),
		Entry("when field type is set (AC-2.7)",
			func(p map[string]any) { nested(p, "dimensions")["field"] = "scalar" }),
		Entry("when feature direction is set (AC-2.7)",
			func(p map[string]any) { nested(p, "dimensions")["direction"] = "read" }),
		Entry("when the constructor shapes carry a field outside the schema",
			func(p map[string]any) { nested(nested(p, "shapes"), "constructor")["NewServer"] = 1 }),
		Entry("when a not-covered shape is outside the closed list",
			func(p map[string]any) {
				p["not_covered"] = []any{map[string]any{"shape": "func ParseConfig", "count": 1}}
			}),
		Entry("when a not-covered item carries a field outside the schema",
			func(p map[string]any) {
				p["not_covered"] = []any{map[string]any{"shape": "layers", "count": 1, "file": "cmd/main.go"}}
			}),
		Entry("when the licence is only whitespace",
			func(p map[string]any) { p["licence"] = "  " }),
		Entry("when the label is not a lowercase path segment",
			func(p map[string]any) { p["label"] = "acme/widgets" }),
		Entry("when the SHA is not 40-hex",
			func(p map[string]any) { p["sha"] = "main" }),
		Entry("when the identity is not a sha256",
			func(p map[string]any) { p["identity"] = "acme" }),
		Entry("when the extractor version is free text",
			func(p map[string]any) { p["extractor_version"] = "built from acme/widgets" }),
		Entry("when its schema_version is not 1",
			func(p map[string]any) { p["schema_version"] = "2" }),
		Entry("when a required field is missing",
			func(p map[string]any) { delete(p, "not_covered") }),
	)

	When("the bytes are not JSON", func() {
		It("fails validation", func() {
			Expect(evalpatterns.ValidateProfile([]byte("label: stand-in"))).NotTo(Succeed())
		})
	})
})
