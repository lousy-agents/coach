package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func commitDeprecatedProjectConfigRepo() string {
	repo := commitScannableGoRepo()
	commitFile(repo, legacyConfigPath, singleRootPolicyJSON)
	return repo
}

func expectDiagnosticKind(report *codesignal.Report, kind, path string) {
	GinkgoHelper()
	_, ok := diagnosticFor(report, kind, path)
	Expect(ok).To(BeTrue(), "expected diagnostic %s at %s, got %+v", kind, path, report.Diagnostics)
}

func expectNoDiagnosticKind(report *codesignal.Report, kind string) {
	GinkgoHelper()
	Expect(report.Diagnostics).NotTo(ContainElement(HaveField("Kind", kind)), "diagnostics: %+v", report.Diagnostics)
}
