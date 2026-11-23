package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("coach codesignal reads the project policy from disk: a policy-only change under --base (#168 A2a)", func() {
	handlersImportDBRepo := func() (repo string) {
		repo = newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/db/db.go", dbPackageFile)
		commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
		return repo
	}

	for _, mode := range scopePolicyModes {
		if !mode.diff {
			continue
		}

		It("#168 item 7: a violation that only a committed policy change makes visible is existing, not introduced, under "+mode.name, func() {
			repo := handlersImportDBRepo()
			base := commitFile(repo, coachConfigPath, diskPolicyWithoutForbiddenImports)
			commitFile(repo, coachConfigPath, goLayerPolicyConfigJSON)

			report, _ := diskPolicyJSONScan(mode, repo, base)

			violations := diskPolicyLayerViolations(report)
			Expect(violations).To(HaveLen(1))
			Expect(violations[0].Lifecycle).To(Equal(codesignal.Lifecycle("existing")), "both sides read the policy on disk")
			Expect(report.ProjectSummary.IntroducedChanges).To(BeZero())
		})

		It("#168 item 7: a violation that only an uncommitted policy edit makes visible is existing, not introduced, under "+mode.name, func() {
			repo := handlersImportDBRepo()
			base := commitFile(repo, coachConfigPath, diskPolicyWithoutForbiddenImports)
			commitFile(repo, "b.go", "package a\n\nfunc B() {}\n")
			overwriteOnDisk(repo, coachConfigPath, goLayerPolicyConfigJSON)

			report, _ := diskPolicyJSONScan(mode, repo, base)

			violations := diskPolicyLayerViolations(report)
			Expect(violations).To(HaveLen(1))
			Expect(violations[0].Lifecycle).To(Equal(codesignal.Lifecycle("existing")), "both sides read the policy on disk")
			Expect(report.ProjectSummary.IntroducedChanges).To(BeZero())
		})
	}
})
