package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func body_projectTsSetupMatrixAcceptanceTest_withholdsYarnSPackageManagerFindingAsAProjectPac_162() {
	GinkgoT().Setenv("PATH", pathWithStubNode("v24.9.9"))
	repo, head := packageManagerMatrixFixture("yarn")

	readiness, err := codesignalcli.CheckProjectReadiness(repo, head, "")
	Expect(err).NotTo(HaveOccurred())
	Expect(readiness.Checks.Compiler.State).To(Equal(projectreadiness.Fail))
	Expect(readiness.Checks.PackageManager.State).To(Equal(projectreadiness.Fail))
	Expect(readiness.Checks.PackageManager.Code).To(Equal(projectreadiness.GapPackageManagerVersionUnsupported))
	Expect(readiness.Checks.PackageManager.Kind).To(Equal("yarn"))

	menu := codesignalcli.AvailableSetupChoices(*readiness)
	Expect(choiceKinds(menu.Choices)).NotTo(ContainElement(codesignalcli.SetupChoiceProjectPackage), "Yarn must never resolve to an executable project-package choice")
	Expect(withheldKinds(menu.Withheld)).To(ContainElement(codesignalcli.SetupChoiceProjectPackage))
	for _, w := range menu.Withheld {
		if w.Kind == codesignalcli.SetupChoiceProjectPackage {
			Expect(w.Reason).To(Equal(projectreadiness.GapPackageManagerVersionUnsupported))
		}
	}
}
