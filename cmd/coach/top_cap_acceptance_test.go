package main

import (
	"encoding/json"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	cognitiveRule = "complexity.cognitive_complexity"
	nestingRule   = "complexity.max_nesting_depth"
)

func signalRuleAtPath(document map[string]json.RawMessage) []string {
	var signals []struct {
		RuleID string `json:"rule_id"`
		Path   string `json:"path"`
	}
	ExpectWithOffset(1, json.Unmarshal(document["signals"], &signals)).To(Succeed())
	ranked := make([]string, 0, len(signals))
	for _, signal := range signals {
		ranked = append(ranked, signal.RuleID+"@"+signal.Path)
	}
	return ranked
}

// topCapFixture commits three nested-if functions whose path order (a, m, z)
// is the reverse of their rank. Each emits a cognitive-complexity and a
// max-nesting-depth signal: a.go is medium, m.go and z.go are high.
func topCapFixture() string {
	repo := newTempGitRepo()
	commitFile(repo, "a.go", nestedIfs("alpha", 6))
	commitFile(repo, "m.go", nestedIfs("mu", 8))
	commitFile(repo, "z.go", nestedIfs("zeta", 10))
	return repo
}

var fullTopCapRanking = []string{
	cognitiveRule + "@z.go",
	cognitiveRule + "@m.go",
	nestingRule + "@z.go",
	nestingRule + "@m.go",
	cognitiveRule + "@a.go",
	nestingRule + "@a.go",
}

var _ = Describe("coach codesignal --top", func() {
	When("the fixture is analyzed without a cap", func() {
		It("ranks six signals in an order that opposes their path order", func() {
			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(topCapFixture(), "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(signalRuleAtPath(jsonDocument(stdout))).To(Equal(fullTopCapRanking))
		})
	})

	When("a baseline scan is capped at the single highest-ranked signal", func() {
		It("renders only that signal and states the withheld count and the command to see them all", func() {
			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(topCapFixture(), "--top", "1")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			text := string(stdout)

			Expect(text).To(ContainSubstring("path: z.go"))
			Expect(text).To(ContainSubstring("rule_id: " + cognitiveRule))
			Expect(text).NotTo(ContainSubstring("path: m.go"))
			Expect(text).NotTo(ContainSubstring("path: a.go"))
			Expect(strings.Count(text, "rule_id: ")).To(Equal(1))
			Expect(text).To(ContainSubstring("withheld: 5 signals beyond --top 1 (counts above describe the full analysis); re-run without --top to see all\n"))
			Expect(text).To(ContainSubstring("active signals: 6"))
		})

		It("emits one signal, the withheld count, and a summary and coverage equal to the uncapped run", func() {
			repo := topCapFixture()
			fullStdout, fullStderr, fullExit := runCoachCodesignalBaselineRaw(repo, "--format=json")
			Expect(fullExit).To(Equal(0), "stderr: %s", fullStderr)
			full := jsonDocument(fullStdout)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--top", "1")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			capped := jsonDocument(stdout)

			Expect(signalRuleAtPath(capped)).To(Equal([]string{cognitiveRule + "@z.go"}))
			Expect(capped["summary"]).To(MatchJSON(full["summary"]))
			Expect(capped["coverage"]).To(MatchJSON(full["coverage"]))
			Expect(capped["diagnostics"]).To(MatchJSON(full["diagnostics"]))
			Expect(capped["signals_withheld"]).To(MatchJSON(`{"top":1,"beyond_top":5}`))
		})
	})

	When("the cap is at or above the number of signals", func() {
		It("renders everything, reports zero withheld, and offers no command to see more", func() {
			repo := topCapFixture()

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--top", "6")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			text := string(stdout)
			Expect(strings.Count(text, "rule_id: ")).To(Equal(6))
			Expect(text).To(ContainSubstring("withheld: 0 signals beyond --top 6 (counts above describe the full analysis)\n"))
			Expect(text).NotTo(ContainSubstring("re-run without"))

			jsonStdout, _, jsonExit := runCoachCodesignalBaselineRaw(repo, "--format=json", "--top", "50")
			Expect(jsonExit).To(Equal(0))
			capped := jsonDocument(jsonStdout)
			Expect(signalRuleAtPath(capped)).To(Equal(fullTopCapRanking))
			Expect(capped["signals_withheld"]).To(MatchJSON(`{"top":50,"beyond_top":0}`))
		})
	})

	When("a severity floor and a cap are both supplied", func() {
		It("applies the floor first, then the cap, and reports each count and the total", func() {
			repo := topCapFixture()

			jsonStdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--min-severity", "high", "--top", "2")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			narrowed := jsonDocument(jsonStdout)
			Expect(signalRuleAtPath(narrowed)).To(Equal([]string{cognitiveRule + "@z.go", cognitiveRule + "@m.go"}))
			Expect(narrowed["signals_withheld"]).To(MatchJSON(`{"min_severity":"high","below_min_severity":2,"top":2,"beyond_top":2}`))

			textStdout, textStderr, textExit := runCoachCodesignalBaselineRaw(repo, "--min-severity", "high", "--top", "2")
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			text := string(textStdout)
			Expect(text).To(ContainSubstring("withheld: 4 signals (2 below --min-severity high, 2 beyond --top 2) (counts above describe the full analysis); re-run without --min-severity and --top to see all\n"))
			Expect(text).NotTo(ContainSubstring("path: a.go"))
			Expect(text).NotTo(ContainSubstring("rule_id: " + nestingRule))
		})
	})

	When("the cap narrows a diff comparison", func() {
		It("renders the highest-ranked signal and states the withheld count", func() {
			repo := newTempGitRepo()
			base := commitFile(repo, "seed.go", "package a\n")
			commitFile(repo, "a.go", nestedIfs("alpha", 6))
			commitFile(repo, "m.go", nestedIfs("mu", 8))
			commitFile(repo, "z.go", nestedIfs("zeta", 10))

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--top", "1")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			text := string(stdout)
			Expect(text).To(ContainSubstring("path: z.go"))
			Expect(text).NotTo(ContainSubstring("path: m.go"))
			Expect(text).NotTo(ContainSubstring("path: a.go"))
			Expect(text).To(ContainSubstring("active signals: 6"))
			Expect(text).To(ContainSubstring("withheld: 5 signals beyond --top 1 (counts above describe the full analysis); re-run without --top to see all\n"))
		})
	})

	When("a project finding ranks beyond the cap", func() {
		It("withholds the project change from the rendered findings once and leaves the project summary whole", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			commitFile(repo, "pkg/deep/deep.go", strings.Replace(nestedIfs("deep", 8), "package a", "package deep", 1))
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)

			fullStdout, fullStderr, fullExit := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")
			Expect(fullExit).To(Equal(0), "stderr: %s", fullStderr)
			full := jsonDocument(fullStdout)
			fullReport := decodeCoachReport(fullStdout)
			Expect(fullReport.ProjectChanges).To(HaveLen(1))
			Expect(string(fullReport.ProjectChanges[0].Severity)).To(Equal("advisory"))
			Expect(fullReport.Signals).To(HaveLen(3), "two file-local signals plus the mirrored project change")
			Expect(string(fullReport.Signals[2].Severity)).To(Equal("advisory"), "the project change must rank last")

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json", "--top", "2")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			capped := jsonDocument(stdout)
			Expect(signalRuleAtPath(capped)).To(Equal([]string{cognitiveRule + "@pkg/deep/deep.go", nestingRule + "@pkg/deep/deep.go"}))
			Expect(string(capped["project_changes"])).To(Equal("[]"))
			Expect(capped["project_summary"]).To(MatchJSON(full["project_summary"]))
			Expect(capped["signals_withheld"]).To(MatchJSON(`{"top":2,"beyond_top":1}`))

			textStdout, textStderr, textExit := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--top", "2")
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			Expect(string(textStdout)).NotTo(ContainSubstring("semantic_key:"))
			Expect(string(textStdout)).To(ContainSubstring("withheld: 1 signal beyond --top 2"))

			keptStdout, keptStderr, keptExit := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json", "--top", "3")
			Expect(keptExit).To(Equal(0), "stderr: %s", keptStderr)
			kept := decodeCoachReport(keptStdout)
			Expect(kept.ProjectChanges).To(HaveLen(1))
			Expect(kept.Signals).To(HaveLen(3))
		})
	})

	When("a cap is supplied", func() {
		It("leaves the exit status independent of what the report contains", func() {
			repo := topCapFixture()

			for _, args := range [][]string{
				{},
				{"--top", "1"},
				{"--top", "100"},
				{"--min-severity", "high", "--top", "1"},
			} {
				_, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, args...)
				Expect(exitCode).To(Equal(0), "args %v, stderr: %s", args, stderr)
			}
		})

		DescribeTable("rejects a value that is not a positive integer as a usage error",
			func(value string) {
				stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(topCapFixture(), "--top", value)
				Expect(exitCode).To(Equal(2))
				Expect(stdout).To(BeEmpty())
				Expect(string(stderr)).To(ContainSubstring(fmt.Sprintf("coach: invalid --top value %q: must be a positive integer", value)))
			},
			Entry("zero", "0"),
			Entry("negative", "-1"),
			Entry("non-numeric", "x"),
			Entry("fractional", "1.5"),
			Entry("empty", ""),
			Entry("out of range", "99999999999999999999"),
		)

		It("documents the flag in the usage text", func() {
			stdout, _, exitCode := runCoachCodesignalBaselineRaw(topCapFixture(), "--help")
			Expect(exitCode).To(Equal(0))
			Expect(strings.Split(string(stdout), "\n")[0]).To(ContainSubstring("[--top N]"))
		})
	})

	When("the flag is omitted", func() {
		It("emits no withheld count in text or JSON", func() {
			repo := topCapFixture()

			textStdout, _, textExit := runCoachCodesignalBaselineRaw(repo)
			Expect(textExit).To(Equal(0))
			Expect(string(textStdout)).NotTo(ContainSubstring("withheld"))

			jsonStdout, _, jsonExit := runCoachCodesignalBaselineRaw(repo, "--format=json")
			Expect(jsonExit).To(Equal(0))
			Expect(jsonDocument(jsonStdout)).NotTo(HaveKey("signals_withheld"))
		})
	})

	When("a non-scan mode is combined with the cap", func() {
		It("rejects it rather than silently ignoring it", func() {
			repo := topCapFixture()

			_, checkStderr, checkExit := runCoachCodesignalBaselineRaw(repo, "--check-project", "--project-language", "typescript", "--top", "1")
			Expect(checkExit).To(Equal(2))
			Expect(string(checkStderr)).To(ContainSubstring("coach: --check-project cannot be combined with --top"))

			_, prepareStderr, prepareExit := runCoachCodesignalBaselineRaw(repo, "--prepare-compiler", "--project-language", "typescript", "--top", "1")
			Expect(prepareExit).To(Equal(2))
			Expect(string(prepareStderr)).To(ContainSubstring("coach: --prepare-compiler cannot be combined with --top"))

			_, suggestStderr, suggestExit := runCoachCodesignalBaselineRaw(repo, "--suggest-project-config", "--top", "1")
			Expect(suggestExit).To(Equal(2))
			Expect(string(suggestStderr)).To(ContainSubstring("coach: --suggest-project-config cannot be combined with --top"))
		})
	})
})
