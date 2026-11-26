package evalpatterns_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalpatterns"
)

// at walks profile along path, where "0" indexes into an array, and returns
// the object that holds the last key.
func at(profile map[string]any, path string) (map[string]any, string) {
	keys := strings.Split(path, ".")
	var node any = profile
	for _, key := range keys[:len(keys)-1] {
		if list, ok := node.([]any); ok {
			node = list[0]
			continue
		}
		node = node.(map[string]any)[key]
	}
	return node.(map[string]any), keys[len(keys)-1]
}

// Every key the schema requires, written out rather than read from the
// schema, so that dropping one from the schema fails its row.
var requiredProfileKeys = []string{
	"schema_version", "identity", "label", "sha", "licence", "extractor_version",
	"roles", "dimensions", "shapes", "not_covered",
	"roles.api.packages",
	"dimensions.layers", "dimensions.entry", "dimensions.depth",
	"dimensions.trap", "dimensions.field", "dimensions.direction",
	"dimensions.entry.http_handler", "dimensions.entry.method", "dimensions.entry.function",
	"shapes.nesting", "shapes.cognitive", "shapes.branch_density", "shapes.constructor",
	"shapes.constructor.assigns_only", "shapes.constructor.does_work",
	"not_covered.0.shape", "not_covered.0.count",
}

var _ = Describe("the profile schema's required keys and types (AC-1.4)", func() {
	for _, path := range requiredProfileKeys {
		It("refuses a profile without "+path, func() {
			profile := validProfile()
			object, key := at(profile, path)
			delete(object, key)

			Expect(evalpatterns.ValidateProfile(profileJSON(profile))).NotTo(Succeed())
		})
	}

	DescribeTable("refuses a value of the wrong type",
		func(path string, value any) {
			profile := validProfile()
			object, key := at(profile, path)
			object[key] = value

			Expect(evalpatterns.ValidateProfile(profileJSON(profile))).NotTo(Succeed())
		},
		Entry("identity as a number", "identity", 7),
		Entry("label as a number", "label", 7),
		Entry("sha as a number", "sha", 7),
		Entry("licence as a number", "licence", 7),
		Entry("extractor_version as a number", "extractor_version", 7),
		Entry("roles as a list", "roles", []any{}),
		Entry("a role as a string", "roles.api", "api"),
		Entry("packages as a list", "roles.api.packages", []any{}),
		Entry("dimensions as a list", "dimensions", []any{}),
		Entry("layers as a list", "dimensions.layers", []any{}),
		Entry("entry as a list", "dimensions.entry", []any{}),
		Entry("an entry count as a string", "dimensions.entry.method", "4"),
		Entry("an entry count as a fraction", "dimensions.entry.function", 1.5),
		Entry("an entry count below zero", "dimensions.entry.http_handler", -1),
		Entry("depth as a list", "dimensions.depth", []any{}),
		Entry("shapes as a list", "shapes", []any{}),
		Entry("a distribution count as a string", "shapes.nesting.1", "5"),
		Entry("cognitive as a list", "shapes.cognitive", []any{}),
		Entry("branch_density as a list", "shapes.branch_density", []any{}),
		Entry("constructor as a list", "shapes.constructor", []any{}),
		Entry("a constructor count as a string", "shapes.constructor.assigns_only", "1"),
		Entry("a constructor count below zero", "shapes.constructor.does_work", -1),
		Entry("not_covered as an object", "not_covered", map[string]any{}),
		Entry("a not-covered count of zero", "not_covered.0.count", 0),
		Entry("a not-covered count as a string", "not_covered.0.count", "1"),
	)
})
