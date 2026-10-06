package main

import (
	"bytes"
	"encoding/json"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("coach codesignal", func() {
	When("--format is omitted and an introduced signal is present", func() {
		It("renders a text report with all required labels and no ANSI escapes", func() {
			repo := newTempGitRepo()
			base := "package a\n\nfunc Get(input *int) int {\n\treturn *input\n}\n"
			head := base + "\nfunc Update(input *int) {\n\t*input = 1\n}\n"
			initialSHA := commitFile(repo, "a.go", base)
			commitFile(repo, "a.go", head)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA)
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			text := string(stdout)
			Expect(text).To(ContainSubstring("files analyzed"))
			Expect(text).To(ContainSubstring("active signals"))
			Expect(text).To(ContainSubstring("diagnostics"))
			Expect(text).To(ContainSubstring("path: a.go"))
			Expect(text).To(ContainSubstring("line: 8"))
			Expect(text).To(ContainSubstring("lifecycle: introduced"))
			Expect(text).To(ContainSubstring("changed: true"))
			Expect(text).To(ContainSubstring("evidence"))
			Expect(text).To(ContainSubstring("why it matters"))
			Expect(text).To(ContainSubstring("recommendation"))
			Expect(text).NotTo(ContainSubstring("\x1b["))
		})
	})

	When("a signal is rendered in both text and JSON formats", func() {
		It("uses a 1-based text line while retaining the 0-based JSON location and renders diagnostics after signals", func() {
			repo := newTempGitRepo()
			base := "package a\n\nfunc Get(input *int) int {\n\treturn *input\n}\n"
			head := base + "\nfunc Update(input *int) {\n\t*input = 1\n}\n"
			initialSHA := commitFile(repo, "a.go", base)
			commitFile(repo, "a.go", head)
			commitFile(repo, "empty.go", "")

			text, textStderr, textExitCode := runCoachCodesignalRaw(repo, initialSHA)
			Expect(textExitCode).To(Equal(0), "stderr: %s", textStderr)
			Expect(string(text)).To(ContainSubstring("line: 8"))
			Expect(strings.Index(string(text), "path: a.go")).To(BeNumerically("<", strings.Index(string(text), "Diagnostics:")))

			jsonOutput, jsonStderr, jsonExitCode := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(jsonExitCode).To(Equal(0), "stderr: %s", jsonStderr)
			var report codesignal.Report
			Expect(json.Unmarshal(jsonOutput, &report)).To(Succeed())
			signals := signalsForPath(&report, "a.go")
			Expect(signals).To(HaveLen(1))
			Expect(signals[0].Location.StartRow).To(Equal(uint(7)))
		})
	})

	When("--format=text and there are no active signals but there is a diagnostic", func() {
		It("renders the diagnostics section and a verdict qualified as incomplete", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
			commitFile(repo, "a.go", "package a\n\n// updated\nfunc A() {}\n")
			commitFile(repo, "empty.go", "")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--format=text")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			text := string(stdout)
			Expect(text).To(ContainSubstring("No active CodeSignal findings, but the analysis is incomplete"))
			Expect(text).To(ContainSubstring("empty.go"))
			Expect(text).To(ContainSubstring("empty_content"))
		})
	})

	When("--format=json", func() {
		var (
			stdout     []byte
			headSHA    string
			initialSHA string
		)

		BeforeEach(func() {
			repo := newTempGitRepo()
			initialSHA = commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
			commitFile(repo, "b.go", "package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")
			headSHA = commitFile(repo, "binary.go", "package binary\x00")

			var stderr []byte
			var exitCode int
			stdout, stderr, exitCode = runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		})

		It("writes exactly one unwrapped report document followed by exactly one newline", func() {
			Expect(bytes.Count(stdout, []byte("\n"))).To(Equal(1))

			var report codesignal.Report
			decoder := json.NewDecoder(bytes.NewReader(stdout))
			Expect(decoder.Decode(&report)).To(Succeed())
			Expect(decoder.More()).To(BeFalse())
			Expect(report.SchemaVersion).NotTo(BeEmpty())
			Expect(report.Scope.Repository).To(BeEmpty())
			Expect(report.Scope.Revision).To(Equal(headSHA))
			Expect(report.Scope.Base).To(Equal(initialSHA))

			var document map[string]json.RawMessage
			Expect(json.Unmarshal(stdout, &document)).To(Succeed())
			Expect(document).To(HaveKey("schema_version"))
			Expect(document).To(HaveKey("scope"))
			Expect(document).To(HaveKey("summary"))
			Expect(document).To(HaveKey("signals"))
			Expect(document).To(HaveKey("diagnostics"))
			Expect(document).To(HaveKey("coverage"))
			Expect(document).To(HaveLen(6), "JSON mode must not add CLI-only fields around codesignal.Report")
		})

		It("emits empty coverage.excluded and coverage.unsupported arrays when scope excludes nothing", func() {
			var document map[string]json.RawMessage
			Expect(json.Unmarshal(stdout, &document)).To(Succeed())

			var coverage map[string]json.RawMessage
			Expect(json.Unmarshal(document["coverage"], &coverage)).To(Succeed())
			Expect(coverage).To(HaveKey("excluded"))
			Expect(coverage).To(HaveKey("unsupported"))
			Expect(string(coverage["excluded"])).To(Equal("[]"), "coverage.excluded must be present and empty when scope excludes nothing")
			Expect(string(coverage["unsupported"])).To(Equal("[]"), "coverage.unsupported must be present and empty when scope excludes nothing")
		})
	})

	When("every changed file was analyzed and no signals were found", func() {
		It("renders the unqualified no-findings verdict", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
			commitFile(repo, "a.go", "package a\n\nfunc A() {}\n\n// note\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA)
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(string(stdout)).To(ContainSubstring("No active CodeSignal findings.\n"))
			Expect(string(stdout)).NotTo(ContainSubstring("incomplete"))
		})
	})
})
