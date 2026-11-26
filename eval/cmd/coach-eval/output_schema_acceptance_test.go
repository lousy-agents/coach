package main_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// validTrendDocument is a trend document that the committed trend schema accepts. Each negative row
// breaks one constraint of it, and the base spec proves that it is valid before any row runs.
func validTrendDocument() map[string]any {
	return map[string]any{
		"schema_version": "1",
		"cell":           "cell-a",
		"metric":         "results.clean_success",
		"runs":           3,
		"segments": []any{map[string]any{
			"runs":   []any{"run-1", "run-2"},
			"first":  map[string]any{"run_id": "run-1", "value": 0.5},
			"last":   map[string]any{"run_id": "run-2", "value": 0.75},
			"change": 0.25,
		}},
		"breaks": []any{map[string]any{
			"before": "run-3",
			"reason": "solver.model differs",
			"fields": []any{"solver.model"},
		}},
	}
}

// validCompareDocument is a compare document that the committed compare schema accepts.
func validCompareDocument() map[string]any {
	return map[string]any{
		"schema_version":    "1",
		"comparable":        false,
		"runs":              []any{"run-a", "run-b"},
		"differing_fields":  []any{"solver.model"},
		"warnings":          []any{},
		"not_complete_runs": []any{map[string]any{"run_id": "run-b", "state": "paused"}},
	}
}

func firstSegmentOf(document map[string]any) map[string]any {
	return document["segments"].([]any)[0].(map[string]any)
}

func firstBreakOf(document map[string]any) map[string]any {
	return document["breaks"].([]any)[0].(map[string]any)
}

func firstNotCompleteOf(document map[string]any) map[string]any {
	return document["not_complete_runs"].([]any)[0].(map[string]any)
}

func marshalDocument(document map[string]any) []byte {
	data, err := json.Marshal(document)
	Expect(err).NotTo(HaveOccurred())
	return data
}

var _ = Describe("the committed output schemas", func() {
	Describe("trend output", func() {
		It("accepts the base document, so each negative row fails for its own constraint", func() {
			Expect(schemaValidationError(trendOutputSchemaPath, marshalDocument(validTrendDocument()))).To(Succeed())
		})

		DescribeTable("rejects a document that breaks one constraint", func(breakConstraint func(map[string]any)) {
			document := validTrendDocument()
			breakConstraint(document)

			Expect(schemaValidationError(trendOutputSchemaPath, marshalDocument(document))).NotTo(Succeed())
		},
			Entry("has schema_version 2", func(d map[string]any) { d["schema_version"] = "2" }),
			Entry("lacks schema_version", func(d map[string]any) { delete(d, "schema_version") }),
			Entry("has an extra top-level key", func(d map[string]any) { d["extra"] = true }),
			Entry("has a negative run count", func(d map[string]any) { d["runs"] = -1 }),
			Entry("has a run count as a string", func(d map[string]any) { d["runs"] = "3" }),
			Entry("has an empty cell", func(d map[string]any) { d["cell"] = "" }),
			Entry("has a segment with no runs", func(d map[string]any) { firstSegmentOf(d)["runs"] = []any{} }),
			Entry("has a segment with an extra key", func(d map[string]any) { firstSegmentOf(d)["extra"] = 1 }),
			Entry("lacks a segment change", func(d map[string]any) { delete(firstSegmentOf(d), "change") }),
			Entry("has a segment change as a string", func(d map[string]any) { firstSegmentOf(d)["change"] = "0.25" }),
			Entry("has a first run without run_id", func(d map[string]any) {
				delete(firstSegmentOf(d)["first"].(map[string]any), "run_id")
			}),
			Entry("has a last run value as a string", func(d map[string]any) {
				firstSegmentOf(d)["last"].(map[string]any)["value"] = "0.75"
			}),
			Entry("lacks break fields", func(d map[string]any) { delete(firstBreakOf(d), "fields") }),
			Entry("has a break field that is not a string", func(d map[string]any) { firstBreakOf(d)["fields"] = []any{1} }),
			Entry("has an empty break reason", func(d map[string]any) { firstBreakOf(d)["reason"] = "" }),
			Entry("lacks cell", func(d map[string]any) { delete(d, "cell") }),
			Entry("lacks metric", func(d map[string]any) { delete(d, "metric") }),
			Entry("lacks runs", func(d map[string]any) { delete(d, "runs") }),
			Entry("lacks segments", func(d map[string]any) { delete(d, "segments") }),
			Entry("lacks breaks", func(d map[string]any) { delete(d, "breaks") }),
			Entry("has an empty metric", func(d map[string]any) { d["metric"] = "" }),
			Entry("has a run count that is not an integer", func(d map[string]any) { d["runs"] = 1.5 }),
			Entry("has segments that are not an array", func(d map[string]any) { d["segments"] = "x" }),
			Entry("has a first run with an extra key", func(d map[string]any) {
				firstSegmentOf(d)["first"].(map[string]any)["extra"] = 1
			}),
			Entry("has a first run without value", func(d map[string]any) {
				delete(firstSegmentOf(d)["first"].(map[string]any), "value")
			}),
			Entry("has an empty first run_id", func(d map[string]any) {
				firstSegmentOf(d)["first"].(map[string]any)["run_id"] = ""
			}),
			Entry("has a segment without runs", func(d map[string]any) { delete(firstSegmentOf(d), "runs") }),
			Entry("has a segment without first", func(d map[string]any) { delete(firstSegmentOf(d), "first") }),
			Entry("has a segment without last", func(d map[string]any) { delete(firstSegmentOf(d), "last") }),
			Entry("has an empty segment run id", func(d map[string]any) { firstSegmentOf(d)["runs"] = []any{""} }),
			Entry("has a break with an extra key", func(d map[string]any) { firstBreakOf(d)["extra"] = 1 }),
			Entry("has a break without before", func(d map[string]any) { delete(firstBreakOf(d), "before") }),
			Entry("has a break without reason", func(d map[string]any) { delete(firstBreakOf(d), "reason") }),
			Entry("has an empty break before", func(d map[string]any) { firstBreakOf(d)["before"] = "" }),
			Entry("has an empty break field", func(d map[string]any) { firstBreakOf(d)["fields"] = []any{""} }),
		)
	})

	Describe("compare output", func() {
		It("accepts the base document, so each negative row fails for its own constraint", func() {
			Expect(schemaValidationError(compareOutputSchemaPath, marshalDocument(validCompareDocument()))).To(Succeed())
		})

		DescribeTable("rejects a document that breaks one constraint", func(breakConstraint func(map[string]any)) {
			document := validCompareDocument()
			breakConstraint(document)

			Expect(schemaValidationError(compareOutputSchemaPath, marshalDocument(document))).NotTo(Succeed())
		},
			Entry("has schema_version 2", func(d map[string]any) { d["schema_version"] = "2" }),
			Entry("lacks comparable", func(d map[string]any) { delete(d, "comparable") }),
			Entry("has comparable as a string", func(d map[string]any) { d["comparable"] = "false" }),
			Entry("lists one run", func(d map[string]any) { d["runs"] = []any{"run-a"} }),
			Entry("lists three runs", func(d map[string]any) { d["runs"] = []any{"run-a", "run-b", "run-c"} }),
			Entry("has null warnings", func(d map[string]any) { d["warnings"] = nil }),
			Entry("has null differing_fields", func(d map[string]any) { d["differing_fields"] = nil }),
			Entry("has an extra key in a not-complete run", func(d map[string]any) { firstNotCompleteOf(d)["extra"] = 1 }),
			Entry("lacks a not-complete run's state", func(d map[string]any) { delete(firstNotCompleteOf(d), "state") }),
			Entry("lacks schema_version", func(d map[string]any) { delete(d, "schema_version") }),
			Entry("lacks runs", func(d map[string]any) { delete(d, "runs") }),
			Entry("lacks differing_fields", func(d map[string]any) { delete(d, "differing_fields") }),
			Entry("lacks warnings", func(d map[string]any) { delete(d, "warnings") }),
			Entry("lacks not_complete_runs", func(d map[string]any) { delete(d, "not_complete_runs") }),
			Entry("has an empty run id", func(d map[string]any) { d["runs"] = []any{"", "run-b"} }),
			Entry("has an empty differing field", func(d map[string]any) { d["differing_fields"] = []any{""} }),
			Entry("has an empty warning", func(d map[string]any) { d["warnings"] = []any{""} }),
			Entry("has not_complete_runs that are not an array", func(d map[string]any) { d["not_complete_runs"] = "x" }),
			Entry("has a not-complete run without run_id", func(d map[string]any) { delete(firstNotCompleteOf(d), "run_id") }),
			Entry("has a not-complete run with an empty run_id", func(d map[string]any) { firstNotCompleteOf(d)["run_id"] = "" }),
			Entry("has a not-complete run with an empty state", func(d map[string]any) { firstNotCompleteOf(d)["state"] = "" }),
			Entry("has an extra top-level key", func(d map[string]any) { d["extra"] = true }),
		)
	})
})
