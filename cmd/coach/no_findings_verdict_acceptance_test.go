package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the zero-active-signal text verdict", func() {
	When("a report has zero active signals, zero project changes, and zero diagnostics", func() {
		It("prints the unqualified verdict with no incomplete-analysis qualifier", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
			commitFile(repo, "a.go", "package a\n\n// note\nfunc A() {}\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--format=text")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			text := string(stdout)
			Expect(text).To(ContainSubstring("No active CodeSignal findings.\n"))
			Expect(text).NotTo(ContainSubstring("incomplete"), "a genuinely clean run must not read as if the analysis were incomplete")
		})
	})

	When("a file is renamed with no content change, discarding a diagnosable finding", func() {
		var incompleteText string

		BeforeEach(func() {
			repo := newTempGitRepo()

			// The file being renamed already contains finding-triggering
			// content, unchanged, so a rename-only diff (unsupported_change_type)
			// is what discards it -- see the sibling It below for the control
			// proving this content is a real finding trigger.
			oldContent := "package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n"
			initialSHA := commitFile(repo, "old.go", oldContent)
			renameFile(repo, "old.go", "new.go")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--format=text")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			incompleteText = string(stdout)
		})

		It("prints a verdict distinct from a genuinely clean run, naming that the analysis did not complete", func() {
			// Control: proves this exact hidden-input-mutation content, when
			// actually analyzed (no rename), produces a signal -- so the
			// rename fixture above is skipping a real finding, not a no-op.
			controlRepo := newTempGitRepo()
			controlBase := "package a\n\nfunc Get(input *int) int {\n\treturn *input\n}\n"
			controlHead := controlBase + "\nfunc Update(input *int) {\n\t*input = 1\n}\n"
			controlInitialSHA := commitFile(controlRepo, "old.go", controlBase)
			commitFile(controlRepo, "old.go", controlHead)
			controlReport, controlStderr := runCoachCodesignal(controlRepo, controlInitialSHA)
			Expect(controlStderr).To(BeEmpty())
			Expect(signalsForPath(controlReport, "old.go")).NotTo(BeEmpty(), "the fixture content must be a real finding trigger, or this test proves nothing")

			cleanRepo := newTempGitRepo()
			cleanInitialSHA := commitFile(cleanRepo, "a.go", "package a\n\nfunc A() {}\n")
			commitFile(cleanRepo, "a.go", "package a\n\n// note\nfunc A() {}\n")
			cleanStdout, cleanStderr, cleanExitCode := runCoachCodesignalRaw(cleanRepo, cleanInitialSHA, "--format=text")
			Expect(cleanExitCode).To(Equal(0), "stderr: %s", cleanStderr)
			cleanText := string(cleanStdout)

			cleanVerdict := verdictSentence(cleanText)
			incompleteVerdict := verdictSentence(incompleteText)
			Expect(incompleteVerdict).NotTo(Equal(cleanVerdict), "an incomplete-analysis run must not print the exact same verdict sentence as a genuinely clean run")

			Expect(incompleteText).To(ContainSubstring("No active CodeSignal findings"), "verdict must still state that no findings were produced")
			Expect(incompleteText).To(ContainSubstring("incomplete"), "verdict must state the analysis did not complete")
			Expect(incompleteVerdict).To(ContainSubstring("1 path"), "the verdict sentence itself (not just the summary line's diagnostics count) must name the number of affected paths")
		})

		It("renders the qualified verdict before the Diagnostics block", func() {
			diagnosticsIdx := strings.Index(incompleteText, "Diagnostics:")
			Expect(diagnosticsIdx).To(BeNumerically(">", 0), "expected a Diagnostics section")
			verdictIdx := strings.Index(incompleteText, "incomplete")
			Expect(verdictIdx).To(BeNumerically("<", diagnosticsIdx), "the qualified verdict must render before the diagnostics block")
		})
	})
})

// verdictSentence extracts the first line of rendered text, which is where
// the zero-active-signal verdict (qualified or not) always appears.
func verdictSentence(text string) string {
	lines := strings.SplitN(text, "\n", 3)
	if len(lines) < 2 {
		return text
	}
	return lines[1]
}
