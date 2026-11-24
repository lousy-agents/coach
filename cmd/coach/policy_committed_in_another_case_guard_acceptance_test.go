package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

const anotherCaseClause = "in another case than"

// expectAbsentWithoutTheCaseClause checks that a deleted policy committed at its own spelling is disclosed as absent,
// and that neither the diagnostic nor the WARNING says it is in another case.
func expectAbsentWithoutTheCaseClause(mode scopePolicyMode, repo, base, policyPath string) {
	GinkgoHelper()
	report, _ := diskPolicyJSONScan(mode, repo, base)
	absent, found := diagnosticFor(report, codesignal.DiagKindProjectPolicyAbsentOnDisk, policyPath)
	Expect(found).To(BeTrue(), "diagnostics: %+v", report.Diagnostics)
	Expect(absent.Message).NotTo(ContainSubstring(codesignal.AbsentPolicyInAnotherCase), "diagnostic: %+v", absent)
	text := diskPolicyTextScan(mode, repo, base)
	warnings := warningLinesNaming(text, policyPath, "absent on disk")
	Expect(warnings).NotTo(BeEmpty(), "text:\n%s", text)
	Expect(warnings).NotTo(ContainElement(ContainSubstring(anotherCaseClause)), "text:\n%s", text)
}

// Each clause of the guard that adds "in another case than" alone keeps it off a policy committed at its own spelling,
// and a policy committed only in another case is disclosed as absent whatever the scan read in its place.
var _ = Describe("the in-another-case clause names only a policy committed in another case than the default path (#168)", func() {
	for _, mode := range scopePolicyModes {
		It("#168: under "+mode.name+", a deleted default policy at the default spelling carries no clause", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, goLayerPolicyConfigJSON})
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())

			expectAbsentWithoutTheCaseClause(mode, repo, base, coachConfigPath)
		})

		It("#168: under "+mode.name+", a deleted root project.json that HEAD selects carries no clause", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{legacyConfigPath, goLayerPolicyConfigJSON})
			Expect(os.Remove(filepath.Join(repo, legacyConfigPath))).To(Succeed())

			expectAbsentWithoutTheCaseClause(mode, repo, base, legacyConfigPath)
		})

		It("#168: under "+mode.name+", a deleted policy committed in another case beside a usable root project.json is disclosed, and the scan says it used project.json instead", func() {
			repo, base := diskPolicyRepo(
				scopePolicyFile{committedInAnotherCase, diskPolicyWithoutForbiddenImports},
				scopePolicyFile{legacyConfigPath, goLayerPolicyConfigJSON},
			)
			Expect(os.Remove(filepath.Join(repo, committedInAnotherCase))).To(Succeed())

			report, _ := diskPolicyJSONScan(mode, repo, base)
			text := diskPolicyTextScan(mode, repo, base)

			absent, found := diagnosticFor(report, codesignal.DiagKindProjectPolicyAbsentOnDisk, committedInAnotherCase)
			Expect(found).To(BeTrue(), "diagnostics: %+v", report.Diagnostics)
			Expect(absent.Message).To(ContainSubstring(codesignal.AbsentPolicyInAnotherCase+" "+coachConfigPath), "diagnostic: %+v", absent)
			Expect(warningLinesNaming(text, committedInAnotherCase, anotherCaseClause+" "+coachConfigPath, "used project.json instead")).NotTo(BeEmpty(), "text:\n%s", text)
		})

		It("#168: under "+mode.name+", a directory committed as .lousy-agents/Coach.json is no policy in another case", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{committedInAnotherCase + "/policy.json", goLayerPolicyConfigJSON})
			Expect(os.RemoveAll(filepath.Join(repo, committedInAnotherCase))).To(Succeed())

			report, _ := diskPolicyJSONScan(mode, repo, base)

			expectNoDiagnosticKind(report, codesignal.DiagKindProjectPolicyAbsentOnDisk)
		})

		It("#168: under "+mode.name+", of several spellings committed in another case, the first in byte order is named", func() {
			repo, base := diskPolicyRepo(
				scopePolicyFile{committedInAnotherCase, goLayerPolicyConfigJSON},
				scopePolicyFile{".lousy-agents/COACH.json", goLayerPolicyConfigJSON},
			)
			Expect(os.Remove(filepath.Join(repo, committedInAnotherCase))).To(Succeed())
			Expect(os.Remove(filepath.Join(repo, ".lousy-agents/COACH.json"))).To(Succeed())

			report, _ := diskPolicyJSONScan(mode, repo, base)

			expectDiagnosticKind(report, codesignal.DiagKindProjectPolicyAbsentOnDisk, ".lousy-agents/COACH.json")
		})
	}
})
