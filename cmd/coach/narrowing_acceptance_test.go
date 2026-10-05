package main

import (
	"encoding/json"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

const withheldNote = "; counts above describe the full analysis"

// withheldLine returns the single `withheld:` line of a text report.
func withheldLine(text string) string {
	var found []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "withheld: ") {
			found = append(found, line)
		}
	}
	ExpectWithOffset(1, found).To(HaveLen(1), "report text:\n%s", text)
	return found[0]
}

type narrowedAccounting struct {
	Summary struct {
		ActiveSignals int `json:"active_signals"`
	} `json:"summary"`
	Signals         []json.RawMessage `json:"signals"`
	SignalsWithheld struct {
		BelowMinSeverity int `json:"below_min_severity"`
		BeyondTop        int `json:"beyond_top"`
	} `json:"signals_withheld"`
}

var _ = Describe("coach codesignal narrowing: the text line that says how to see everything", func() {
	When("a severity floor alone withholds signals", func() {
		It("ends the withheld line with the command that re-runs without the floor", func() {
			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(severityFloorFixture(), "--min-severity", "high")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(withheldLine(string(stdout))).To(Equal(
				"withheld: 2 signals below --min-severity high" + withheldNote + "; see all: coach codesignal --baseline"))
		})
	})

	When("a cap alone withholds signals", func() {
		It("ends the withheld line with the command that re-runs without the cap", func() {
			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(topCapFixture(), "--top", "1")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(withheldLine(string(stdout))).To(Equal(
				"withheld: 5 signals beyond --top 1" + withheldNote + "; see all: coach codesignal --baseline"))
		})
	})

	When("a floor withholds signals and a cap beside it withholds none", func() {
		DescribeTable("the command that shows everything drops both narrowing flags, whatever their spelling",
			func(narrowing ...string) {
				args := append([]string{"--scope", "all"}, narrowing...)

				stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(severityFloorFixture(), args...)

				Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
				Expect(withheldLine(string(stdout))).To(Equal(
					"withheld: 2 signals (2 below --min-severity high, 0 beyond --top 5)" + withheldNote +
						"; see all: coach codesignal --baseline --scope all"))
			},
			Entry("separate values", "--min-severity", "high", "--top", "5"),
			Entry("inline values", "--min-severity=high", "--top=5"),
			Entry("single-dash flags", "-min-severity", "high", "-top=5"),
			Entry("cap before floor", "--top", "5", "--min-severity", "high"),
		)
	})

	When("a floor and a cap both withhold signals", func() {
		It("reports one parenthetical with both counts and a command without either flag", func() {
			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(topCapFixture(), "--min-severity", "high", "--top", "2")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(withheldLine(string(stdout))).To(Equal(
				"withheld: 4 signals (2 below --min-severity high, 2 beyond --top 2)" + withheldNote +
					"; see all: coach codesignal --baseline"))
		})
	})

	When("nothing was withheld", func() {
		It("offers no command to see more", func() {
			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(topCapFixture(), "--top", "6")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(withheldLine(string(stdout))).To(Equal("withheld: 0 signals beyond --top 6" + withheldNote))
		})
	})

	When("another argument needs shell quoting", func() {
		It("prints a command that survives a shell, quoting the argument", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			commitFile(repo, "my config.json", goLayerPolicyConfigJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "my config.json", "--min-severity", "medium")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(withheldLine(string(stdout))).To(Equal(
				"withheld: 1 signal below --min-severity medium" + withheldNote +
					"; see all: coach codesignal --baseline --project-config 'my config.json'"))
		})
	})

	When("the report is JSON", func() {
		It("carries no command, only the counts", func() {
			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(severityFloorFixture(), "--format=json", "--min-severity", "high")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(string(stdout)).NotTo(ContainSubstring("see all"))
			Expect(jsonDocument(stdout)["signals_withheld"]).To(MatchJSON(`{"min_severity":"high","below_min_severity":2}`))
		})
	})
})

var _ = Describe("coach codesignal narrowing: accounting for every active signal", func() {
	DescribeTable("summary.active_signals equals the rendered signals plus everything withheld",
		func(repo func() string, narrowing ...string) {
			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo(), append([]string{"--format=json"}, narrowing...)...)
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var report narrowedAccounting
			Expect(json.Unmarshal(stdout, &report)).To(Succeed())

			withheld := report.SignalsWithheld.BelowMinSeverity + report.SignalsWithheld.BeyondTop
			Expect(withheld).To(BeNumerically(">", 0), "the narrowing must withhold something for the identity to be tested")
			Expect(report.Summary.ActiveSignals).To(Equal(len(report.Signals) + withheld))
		},
		Entry("severity floor only", severityFloorFixture, "--min-severity", "high"),
		Entry("cap only", topCapFixture, "--top", "2"),
		Entry("floor and cap", topCapFixture, "--min-severity", "high", "--top", "1"),
	)
})

var _ = Describe("coach codesignal narrowing with --fail-on-incomplete-coverage", func() {
	var (
		originalLoadProjectConfig     func(string, string, string) (json.RawMessage, error)
		originalResolveProjectBackend func(string) error
	)

	BeforeEach(func() {
		originalLoadProjectConfig = loadProjectConfig
		originalResolveProjectBackend = resolveProjectBackend
		DeferCleanup(func() {
			loadProjectConfig = originalLoadProjectConfig
			resolveProjectBackend = originalResolveProjectBackend
		})
		loadProjectConfig = func(string, string, string) (json.RawMessage, error) {
			return json.RawMessage(`{"schema_version":"1","roots":["."]}`), nil
		}
		resolveProjectBackend = func(string) error {
			return &codesignalcli.ProjectBackendUnavailableError{
				Message: `coach codesignal: no project-analysis backend is available for language "rust" yet (project_backend_unavailable)`,
			}
		}
	})

	runInRepo := func(repo string, args ...string) ([]byte, int) {
		original, err := os.Getwd()
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Chdir(repo)).To(Succeed())
		defer func() { Expect(os.Chdir(original)).To(Succeed()) }()

		stdout, stderr, exitCode := runInProcess(append([]string{"codesignal", "--baseline", "--project-config", "project.json", "--format=json", "--fail-on-incomplete-coverage"}, args...)...)
		Expect(stderr).To(BeEmpty())
		return stdout, exitCode
	}

	DescribeTable("incomplete required coverage still exits 3 and summary and coverage stay whole",
		func(wantRendered int, wantWithheld string, narrowing ...string) {
			repo := newTempGitRepo()
			commitFile(repo, "tangle.go", nestedIfs("tangle", 6))

			fullStdout, fullExit := runInRepo(repo)
			Expect(fullExit).To(Equal(3))
			full := jsonDocument(fullStdout)
			Expect(signalPaths(full)).To(HaveLen(2), "the fixture must carry signals for narrowing to withhold")

			stdout, exitCode := runInRepo(repo, narrowing...)

			Expect(exitCode).To(Equal(3))
			narrowed := jsonDocument(stdout)
			Expect(signalPaths(narrowed)).To(HaveLen(wantRendered))
			Expect(narrowed["signals_withheld"]).To(MatchJSON(wantWithheld))
			Expect(narrowed["summary"]).To(MatchJSON(full["summary"]))
			Expect(narrowed["coverage"]).To(MatchJSON(full["coverage"]))
			Expect(narrowed["diagnostics"]).To(MatchJSON(full["diagnostics"]))
		},
		Entry("a floor that withholds every signal", 0, `{"min_severity":"high","below_min_severity":2}`, "--min-severity", "high"),
		Entry("a cap that withholds a signal", 1, `{"top":1,"beyond_top":1}`, "--top", "1"),
		Entry("a floor that withholds every signal beside a cap", 0, `{"min_severity":"high","below_min_severity":2,"top":1,"beyond_top":0}`, "--min-severity", "high", "--top", "1"),
	)
})
