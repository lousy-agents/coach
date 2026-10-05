package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// nestedIfs returns a Go function with depth nested ifs, whose cognitive
// complexity is 1+2+...+depth.
func nestedIfs(name string, depth int) string {
	var b strings.Builder
	b.WriteString("package a\n\nfunc " + name + "(n int) {\n")
	for i := 0; i < depth; i++ {
		b.WriteString(strings.Repeat("\t", i+1) + "if n > 0 {\n")
	}
	b.WriteString(strings.Repeat("\t", depth+1) + "return\n")
	for i := depth - 1; i >= 0; i-- {
		b.WriteString(strings.Repeat("\t", i+1) + "}\n")
	}
	b.WriteString("}\n")
	return b.String()
}

// findingBlockWith returns the blank-line-delimited text block that carries
// every requiredLines entry as an exact line, so two findings sharing a path
// (different rules) cannot be confused and assertions cannot be satisfied by
// a line in another block or in the summary.
func findingBlockWith(text string, requiredLines ...string) string {
	for _, block := range strings.Split(text, "\n\n") {
		lines := strings.Split(block, "\n")
		matches := true
		for _, required := range requiredLines {
			if !slices.Contains(lines, required) {
				matches = false
				break
			}
		}
		if matches {
			return block
		}
	}
	Fail(fmt.Sprintf("no text block carries all of %q in:\n%s", requiredLines, text))
	return ""
}

var _ = Describe("coach codesignal text finding identity", func() {
	When("a baseline scan reports file-local findings in the default text format", func() {
		It("prints each finding's own rule_id and severity inside that finding's block", func() {
			repo := newTempGitRepo()
			commitFile(repo, "tangle.go", nestedIfs("tangle", 6))
			commitFile(repo, "deep.go", nestedIfs("deep", 8))

			jsonStdout, jsonStderr, jsonExit := runCoachCodesignalBaselineRaw(repo, "--format=json")
			Expect(jsonExit).To(Equal(0), "stderr: %s", jsonStderr)
			report := decodeCoachReport(jsonStdout)
			evidenceByPath := map[string]string{}
			severityByPath := map[string]string{}
			for _, signal := range report.Signals {
				if signal.RuleID == "complexity.cognitive_complexity" {
					evidenceByPath[signal.Path] = signal.Evidence
					severityByPath[signal.Path] = string(signal.Severity)
				}
			}
			Expect(evidenceByPath).To(HaveKeyWithValue("tangle.go", "cognitive_complexity=21"), "fixture must stay below the escalation multiple")
			Expect(evidenceByPath).To(HaveKeyWithValue("deep.go", "cognitive_complexity=36"), "fixture must sit at or above the escalation multiple")
			Expect(severityByPath).To(HaveKeyWithValue("tangle.go", "medium"))
			Expect(severityByPath).To(HaveKeyWithValue("deep.go", "high"))

			textStdout, textStderr, textExit := runCoachCodesignalBaselineRaw(repo)
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			text := string(textStdout)

			lines := func(block string) []string { return strings.Split(block, "\n") }

			tangleComplexity := findingBlockWith(text, "path: tangle.go", "evidence: cognitive_complexity=21")
			Expect(lines(tangleComplexity)).To(ContainElement("rule_id: complexity.cognitive_complexity"))
			Expect(lines(tangleComplexity)).To(ContainElement("severity: medium"))
			Expect(tangleComplexity).NotTo(ContainSubstring("severity: high"))

			tangleNesting := findingBlockWith(text, "path: tangle.go", "evidence: max_nesting_depth=7")
			Expect(lines(tangleNesting)).To(ContainElement("rule_id: complexity.max_nesting_depth"))
			Expect(lines(tangleNesting)).To(ContainElement("severity: medium"))
			Expect(tangleNesting).NotTo(ContainSubstring("complexity.cognitive_complexity"))

			deepComplexity := findingBlockWith(text, "path: deep.go", "evidence: cognitive_complexity=36")
			Expect(lines(deepComplexity)).To(ContainElement("rule_id: complexity.cognitive_complexity"))
			Expect(lines(deepComplexity)).To(ContainElement("severity: high"))
			Expect(deepComplexity).NotTo(ContainSubstring("severity: medium"))

			deepNesting := findingBlockWith(text, "path: deep.go", "evidence: max_nesting_depth=9")
			Expect(lines(deepNesting)).To(ContainElement("rule_id: complexity.max_nesting_depth"))
			Expect(lines(deepNesting)).To(ContainElement("severity: high"))
		})
	})

	When("a baseline scan reports a project finding in the default text format", func() {
		It("prints severity beside rule_id using the same label spelling as the file-local block", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)

			jsonStdout, jsonStderr, jsonExit := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")
			Expect(jsonExit).To(Equal(0), "stderr: %s", jsonStderr)
			report := decodeCoachReport(jsonStdout)
			Expect(report.ProjectChanges).To(HaveLen(1))
			change := report.ProjectChanges[0]
			Expect(string(change.Severity)).NotTo(BeEmpty())

			textStdout, textStderr, textExit := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json")
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			block := findingBlockWith(string(textStdout), "path: pkg/handlers/handlers.go", "semantic_key: "+change.SemanticKey)
			Expect(strings.Split(block, "\n")).To(ContainElement("rule_id: architecture.layer_violation"))
			Expect(strings.Split(block, "\n")).To(ContainElement("severity: " + string(change.Severity)))
		})
	})

	When("the same repository is rendered as JSON", func() {
		It("emits the signal's severity and rule_id under an unchanged key set, with no text-only labels", func() {
			repo := newTempGitRepo()
			commitFile(repo, "tangle.go", nestedIfs("tangle", 6))

			jsonStdout, jsonStderr, jsonExit := runCoachCodesignalBaselineRaw(repo, "--format=json")
			Expect(jsonExit).To(Equal(0), "stderr: %s", jsonStderr)

			var document struct {
				Signals []map[string]any `json:"signals"`
			}
			Expect(json.Unmarshal(jsonStdout, &document)).To(Succeed())
			var complexity map[string]any
			for _, signal := range document.Signals {
				if signal["rule_id"] == "complexity.cognitive_complexity" {
					complexity = signal
				}
			}
			Expect(complexity).NotTo(BeNil(), "cognitive_complexity signal missing from JSON signals")
			Expect(complexity).To(HaveKeyWithValue("rule_id", "complexity.cognitive_complexity"))
			Expect(complexity).To(HaveKeyWithValue("severity", "medium"))
			Expect(slices.Sorted(maps.Keys(complexity))).To(Equal([]string{
				"category", "changed", "confidence", "coverage_refs", "evidence", "fingerprint", "id", "kind",
				"lifecycle", "location", "machine_evidence", "path", "path_steps", "provenance", "recommendation",
				"related_locations", "rule_id", "rule_version", "severity", "source_scope", "subject",
				"suggested_skill", "why_it_matters",
			}))
			Expect(string(jsonStdout)).NotTo(ContainSubstring("rule_id: "))
			Expect(string(jsonStdout)).NotTo(ContainSubstring("severity: "))
		})
	})
})
