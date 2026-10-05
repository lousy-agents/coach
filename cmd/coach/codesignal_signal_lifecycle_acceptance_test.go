package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("coach codesignal", func() {
	When("head introduces a hidden-input-mutation finding not present at base", func() {
		It("reports exactly one introduced signal", func() {
			repo := newTempGitRepo()
			base := "package a\n\nfunc Get(input *int) int {\n\treturn *input\n}\n"
			head := base + "\nfunc Update(input *int) {\n\t*input = 1\n}\n"
			initialSHA := commitFile(repo, "a.go", base)
			commitFile(repo, "a.go", head)

			report, _ := runCoachCodesignal(repo, initialSHA)

			signals := signalsForPath(report, "a.go")
			Expect(signals).To(HaveLen(1))
			Expect(signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
		})
	})

	When("the same hidden-input-mutation finding is present at base and head", func() {
		It("reports the signal as existing", func() {
			repo := newTempGitRepo()
			base := "package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n"
			head := "package a\n\n// note\nfunc Update(input *int) {\n\t*input = 1\n}\n"
			initialSHA := commitFile(repo, "a.go", base)
			commitFile(repo, "a.go", head)

			report, _ := runCoachCodesignal(repo, initialSHA)

			signals := signalsForPath(report, "a.go")
			Expect(signals).To(HaveLen(1))
			Expect(signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
		})
	})

	When("a file with a hidden-input-mutation finding is removed at head", func() {
		It("reports the signal as resolved", func() {
			repo := newTempGitRepo()
			base := "package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n"
			initialSHA := commitFile(repo, "a.go", base)
			removeFile(repo, "a.go")

			report, _ := runCoachCodesignal(repo, initialSHA)

			signals := signalsForPath(report, "a.go")
			Expect(signals).To(HaveLen(1))
			Expect(signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("resolved")))
		})
	})

	When("a change inserts a new function alongside an untouched one", func() {
		It("marks the inserted finding changed and the untouched finding unchanged", func() {
			repo := newTempGitRepo()
			base := "package a\n\nfunc A(input *int) {\n\t*input = 1\n}\n\nfunc B(input *int) {\n\t*input = 2\n}\n"
			head := "package a\n\nfunc A(input *int) {\n\t*input = 1\n}\n\nfunc C(input *int) {\n\t*input = 3\n}\n\nfunc B(input *int) {\n\t*input = 2\n}\n"
			initialSHA := commitFile(repo, "a.go", base)
			commitFile(repo, "a.go", head)

			report, _ := runCoachCodesignal(repo, initialSHA)

			signals := signalsForPath(report, "a.go")
			Expect(signals).To(HaveLen(3))

			var aSignal, bSignal, cSignal *codesignal.Signal
			for i := range signals {
				switch signals[i].Subject {
				case "A:input":
					aSignal = &signals[i]
				case "B:input":
					bSignal = &signals[i]
				case "C:input":
					cSignal = &signals[i]
				}
			}
			Expect(aSignal).NotTo(BeNil())
			Expect(bSignal).NotTo(BeNil())
			Expect(cSignal).NotTo(BeNil())

			Expect(aSignal.Changed).To(BeFalse())
			Expect(bSignal.Changed).To(BeFalse())
			Expect(cSignal.Changed).To(BeTrue())
			Expect(cSignal.Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
		})
	})

	When("head content has a syntax error", func() {
		It("reports a syntax_errors diagnostic, no signals for that file, and exits 0", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
			commitFile(repo, "a.go", "package a\n\nfunc B(\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var report codesignal.Report
			Expect(json.Unmarshal(stdout, &report)).To(Succeed())

			Expect(hasDiagnostic(&report, "syntax_errors", "a.go")).To(BeTrue())
			Expect(signalsForPath(&report, "a.go")).To(BeEmpty())
			Expect(report.Signals).To(BeEmpty(), "a report with only diagnostics and zero signals is a normal, exit-0 outcome")
		})

		It("does not claim a not-analyzed count when JSON omits files_unanalyzed", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
			commitFile(repo, "a.go", "package a\n\nfunc B(\n")

			jsonOut, jsonErr, jsonExit := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(jsonExit).To(Equal(0), "stderr: %s", jsonErr)
			var report codesignal.Report
			Expect(json.Unmarshal(jsonOut, &report)).To(Succeed(), "stdout should be one JSON report: %s", jsonOut)
			Expect(hasDiagnostic(&report, "syntax_errors", "a.go")).To(BeTrue())
			Expect(report.Summary.FilesAnalyzed).To(BeNumerically(">=", 1), "the syntax-error path must have been analyzed")
			Expect(report.Summary.FilesUnanalyzed).To(Equal(0))
			var document struct {
				Summary struct {
					FilesUnanalyzed *int `json:"files_unanalyzed"`
				} `json:"summary"`
			}
			Expect(json.Unmarshal(jsonOut, &document)).To(Succeed())
			Expect(document.Summary.FilesUnanalyzed).To(BeNil(),
				"JSON must omit files_unanalyzed when the only diagnostic path is in Files")

			textOut, textErr, textExit := runCoachCodesignalRaw(repo, initialSHA)
			Expect(textExit).To(Equal(0), "stderr: %s", textErr)
			verdict := verdictLine(string(textOut))
			Expect(verdict).NotTo(ContainSubstring("not analyzed"),
				"an analyzed path's diagnostic must not be described as unanalyzed; got %q", verdict)
			Expect(verdict).To(ContainSubstring("additional diagnostics were recorded"),
				"FilesUnanalyzed==0 with other diagnostics must keep the fallback clause; got %q", verdict)
		})
	})

	When("base content has a syntax error but head is clean", func() {
		It("reports the head signals as unknown lifecycle plus a base_syntax_errors diagnostic", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc B(\n")
			commitFile(repo, "a.go", "package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")

			report, _ := runCoachCodesignal(repo, initialSHA)

			Expect(hasDiagnostic(report, "base_syntax_errors", "a.go")).To(BeTrue())
			signals := signalsForPath(report, "a.go")
			Expect(signals).To(HaveLen(1))
			Expect(signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("unknown")))
		})
	})

	When("head adds a new file containing a hidden-input-mutation finding", func() {
		It("classifies every signal from that file as introduced", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "seed.go", "package seed\n")
			commitFile(repo, "new.go", "package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")

			report, _ := runCoachCodesignal(repo, initialSHA)

			signals := signalsForPath(report, "new.go")
			Expect(signals).To(HaveLen(1))
			Expect(signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
			Expect(report.Summary.IntroducedSignals).To(Equal(1))
		})
	})

	When("one commit deletes a risky file and adds a structurally different risky file", func() {
		It("classifies the added file's signals as introduced", func() {
			repo := newTempGitRepo()
			deleted := "package gone\n\nfunc Update(input *int) {\n\t*input = 1\n}\n"
			added := `package fresh

func tangle(n int) {
	if n > 0 {
		if n > 1 {
			if n > 2 {
				if n > 3 {
					if n > 4 {
						if n > 5 {
							return
						}
					}
				}
			}
		}
	}
}
`
			initialSHA := commitFile(repo, "gone.go", deleted)

			rmCmd := exec.Command("git", "rm", "gone.go")
			rmCmd.Dir = repo
			output, err := rmCmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "git rm: %s", output)

			Expect(os.WriteFile(filepath.Join(repo, "fresh.go"), []byte(added), 0o644)).To(Succeed())
			addCmd := exec.Command("git", "add", "fresh.go")
			addCmd.Dir = repo
			output, err = addCmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "git add: %s", output)

			commitCmd := exec.Command("git", "commit", "-m", "replace gone.go with fresh.go")
			commitCmd.Dir = repo
			commitCmd.Env = commitEnv
			output, err = commitCmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "git commit: %s", output)

			statusCmd := exec.Command("git", "diff", "--name-status", initialSHA, "HEAD")
			statusCmd.Dir = repo
			statusOut, err := statusCmd.Output()
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.Split(strings.TrimSpace(string(statusOut)), "\n")).To(ConsistOf("A\tfresh.go", "D\tgone.go"))

			report, _ := runCoachCodesignal(repo, initialSHA)

			Expect(hasDiagnostic(report, "unsupported_change_type", "fresh.go")).To(BeFalse())
			Expect(report.Summary.IntroducedSignals).To(BeNumerically(">", 0),
				"an added file must contribute introduced signals; got introduced=%d resolved=%d for %d signals",
				report.Summary.IntroducedSignals, report.Summary.ResolvedSignals, len(report.Signals))

			freshSignals := signalsForPath(report, "fresh.go")
			Expect(freshSignals).NotTo(BeEmpty())
			for i := range freshSignals {
				Expect(freshSignals[i].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
			}

			goneSignals := signalsForPath(report, "gone.go")
			Expect(goneSignals).NotTo(BeEmpty())
			for i := range goneSignals {
				Expect(goneSignals[i].Lifecycle).To(Equal(codesignal.Lifecycle("resolved")))
			}
		})
	})

	When("a modified file's merge-base content cannot be analyzed", func() {
		It("counts every signal in exactly one lifecycle bucket including unknown_signals", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc B(\n")
			commitFile(repo, "a.go", "package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var document struct {
				Signals []json.RawMessage `json:"signals"`
				Summary struct {
					IntroducedSignals int `json:"introduced_signals"`
					ExistingSignals   int `json:"existing_signals"`
					ResolvedSignals   int `json:"resolved_signals"`
					BaselineSignals   int `json:"baseline_signals"`
					UnknownSignals    int `json:"unknown_signals"`
				} `json:"summary"`
			}
			Expect(json.Unmarshal(stdout, &document)).To(Succeed(), "stdout should be one JSON report: %s", stdout)

			bucketSum := document.Summary.IntroducedSignals +
				document.Summary.ExistingSignals +
				document.Summary.ResolvedSignals +
				document.Summary.BaselineSignals +
				document.Summary.UnknownSignals
			Expect(bucketSum).To(Equal(len(document.Signals)),
				"lifecycle buckets must count every signal; got introduced=%d existing=%d resolved=%d baseline=%d unknown=%d for len(signals)=%d",
				document.Summary.IntroducedSignals, document.Summary.ExistingSignals,
				document.Summary.ResolvedSignals, document.Summary.BaselineSignals,
				document.Summary.UnknownSignals, len(document.Signals))
			Expect(document.Summary.UnknownSignals).To(BeNumerically(">", 0),
				"unanalyzable merge-base signals must be counted in summary.unknown_signals")

			var report codesignal.Report
			Expect(json.Unmarshal(stdout, &report)).To(Succeed())
			Expect(hasDiagnostic(&report, "base_syntax_errors", "a.go")).To(BeTrue())
		})
	})
})
