package main

import (
	. "github.com/onsi/gomega"
)

func body_baselineAcceptanceTest_excludesATestOnlyGoFileFromSignalsAndAccountsFor_252() {
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
}
