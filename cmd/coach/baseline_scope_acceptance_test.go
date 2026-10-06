package main

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("coach codesignal --baseline", func() {
	When("--scope=production is the default for a Repository Baseline scan", func() {
		It("excludes a test-only Go file from signals and accounts for it in Coverage.Excluded", func() {
			repo := newTempGitRepo()
			commitFile(repo, "shipping/shipping.go", "package shipping\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")
			commitFile(repo, "shipping/shipping_test.go", "package shipping\n\nfunc TestUpdate(input *int) {\n\t*input = 1\n}\n")

			report, stderr := runCoachCodesignalBaseline(repo)
			Expect(stderr).To(BeEmpty())

			Expect(signalsForPath(report, "shipping/shipping.go")).To(HaveLen(1))
			Expect(signalsForPath(report, "shipping/shipping_test.go")).To(BeEmpty(), "default production scope must exclude a test-only Go file from signals")

			Expect(report.Coverage).NotTo(BeNil())
			foundExcluded := false
			for _, g := range report.Coverage.Excluded {
				if g.Reason == "test_only" && g.Language == "go" {
					foundExcluded = true
				}
			}
			Expect(foundExcluded).To(BeTrue(), "the excluded test-only Go file must be accounted for in Coverage.Excluded")
		})
	})

	When("--scope all is given for a Repository Baseline scan", func() {
		It("records the resolved --scope value in scope.applied_scope", func() {
			repo := newTempGitRepo()
			commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--scope", "all", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var report codesignal.Report
			Expect(json.Unmarshal(stdout, &report)).To(Succeed(), "stdout should be one JSON report: %s", stdout)

			Expect(report.Scope.AppliedScope).To(Equal("all"))
		})
	})

	When("--scope is omitted for a Repository Baseline scan", func() {
		It("records the CLI's default applied scope value in scope.applied_scope", func() {
			repo := newTempGitRepo()
			commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")

			report, stderr := runCoachCodesignalBaseline(repo)
			Expect(stderr).To(BeEmpty())

			Expect(report.Scope.AppliedScope).To(Equal("production"), "the default --scope value used by diff mode must also be recorded for baseline mode")
		})
	})
})
