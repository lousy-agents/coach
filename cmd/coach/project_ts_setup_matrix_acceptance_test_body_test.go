package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectcheck"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
)

func body_projectTsSetupMatrixAcceptanceTest_withholdsYarnSPackageManagerFindingAsAProjectPac_162() {
	GinkgoT().Setenv("PATH", pathWithStubNode("v24.9.9"))
	repo, head := packageManagerMatrixFixture("yarn")

	readiness, err := projectcheck.Run(repo, head, "")
	Expect(err).NotTo(HaveOccurred())
	Expect(readiness.Checks.Compiler.State).To(Equal(projectreadiness.Fail))
	Expect(readiness.Checks.PackageManager.State).To(Equal(projectreadiness.Fail))
	Expect(readiness.Checks.PackageManager.Code).To(Equal(projectreadiness.GapPackageManagerVersionUnsupported))
	Expect(readiness.Checks.PackageManager.Kind).To(Equal("yarn"))

	menu := tssetup.AvailableChoices(*readiness)
	Expect(choiceKinds(menu.Choices)).NotTo(ContainElement(tssetup.ChoiceProjectPackage), "Yarn must never resolve to an executable project-package choice")
	Expect(withheldKinds(menu.Withheld)).To(ContainElement(tssetup.ChoiceProjectPackage))
	for _, w := range menu.Withheld {
		if w.Kind == tssetup.ChoiceProjectPackage {
			Expect(w.Reason).To(Equal(projectreadiness.GapPackageManagerVersionUnsupported))
		}
	}
}
