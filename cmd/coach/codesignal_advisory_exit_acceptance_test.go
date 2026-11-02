package main

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("coach codesignal advisory exit status", func() {
	When("a completed diff analysis reports active signals", func() {
		It("exits 0 in both text and JSON formats, because coach does not gate merges", func() {
			repo := newTempGitRepo()
			base := "package a\n\nfunc Get(input *int) int {\n\treturn *input\n}\n"
			head := base + "\nfunc Update(input *int) {\n\t*input = 1\n}\n"
			initialSHA := commitFile(repo, "a.go", base)
			commitFile(repo, "a.go", head)

			textOutput, textStderr, textExit := runCoachCodesignalRaw(repo, initialSHA)
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			Expect(string(textOutput)).NotTo(ContainSubstring("No active CodeSignal findings"), "an exit 0 only proves the advisory contract when the text run found signals")

			jsonOutput, jsonStderr, jsonExit := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(jsonExit).To(Equal(0), "stderr: %s", jsonStderr)

			var report codesignal.Report
			Expect(json.Unmarshal(jsonOutput, &report)).To(Succeed())
			Expect(report.Signals).NotTo(BeEmpty(), "an exit 0 only proves the advisory contract when the run found signals")
			Expect(string(textOutput)).To(ContainSubstring(report.Signals[0].RuleID), "the text run must report the signal the JSON run found")
		})
	})

	When("a completed repository baseline analysis reports active signals", func() {
		It("exits 0 in both text and JSON formats, because coach does not gate merges", func() {
			repo := newTempGitRepo()
			commitFile(repo, "a.go", "package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")

			textOutput, textStderr, textExit := runCoachCodesignalBaselineRaw(repo)
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			Expect(string(textOutput)).NotTo(ContainSubstring("No active CodeSignal findings"), "an exit 0 only proves the advisory contract when the text run found signals")

			jsonOutput, jsonStderr, jsonExit := runCoachCodesignalBaselineRaw(repo, "--format=json")
			Expect(jsonExit).To(Equal(0), "stderr: %s", jsonStderr)

			var report codesignal.Report
			Expect(json.Unmarshal(jsonOutput, &report)).To(Succeed())
			Expect(report.Signals).NotTo(BeEmpty(), "an exit 0 only proves the advisory contract when the run found signals")
			Expect(string(textOutput)).To(ContainSubstring(report.Signals[0].RuleID), "the text run must report the signal the JSON run found")
		})
	})
})
