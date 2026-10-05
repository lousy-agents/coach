package main

import (
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
)

func body_projectTsSetupChoiceAcceptanceTest_withholdsProjectPackageNamedAndOffersOnlyTheMise_103() {
	readiness := projectreadiness.Result{
		Checks: projectreadiness.Checks{
			PackageManager: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPackageManagerVersionUnsupported, Kind: "yarn"},
			Compiler: projectreadiness.Check{
				State: projectreadiness.Fail,
				Code:  projectreadiness.GapTypescriptCompilerMissing,
			},
		},
		MiseChoices: []projectreadiness.MiseChoice{
			{Kind: "mise_project", Verified: true},
			{Kind: "mise_global", Reason: "mise_unconfigured"},
		},
	}

	menu := tssetup.AvailableChoices(readiness)

	Expect(choiceKinds(menu.Choices)).To(Equal([]tssetup.ChoiceKind{tssetup.ChoiceProjectMise, tssetup.ChoiceCancel}))

	var projectPackageReason string
	for _, w := range menu.Withheld {
		if w.Kind == tssetup.ChoiceProjectPackage {
			projectPackageReason = w.Reason
		}
	}
	Expect(projectPackageReason).To(Equal(projectreadiness.GapPackageManagerVersionUnsupported), "the project adapter must be named, not merely omitted")
}

func body_projectTsSetupChoiceAcceptanceTest_withholdsProjectMiseAsUnverifiableRatherThanOffe_243(passingPackageManager projectreadiness.Check) {
	readiness := projectreadiness.Result{
		Checks: projectreadiness.Checks{
			PackageManager: passingPackageManager,
			Compiler: projectreadiness.Check{
				State: projectreadiness.Fail,
				Code:  projectreadiness.GapTypescriptCompilerMissing,
			},
		},
		MiseChoices: []projectreadiness.MiseChoice{
			{Kind: "mise_project", Reason: "mise_unverifiable"},
			{Kind: "mise_global", Verified: true},
		},
	}

	menu := tssetup.AvailableChoices(readiness)

	Expect(choiceKinds(menu.Choices)).NotTo(ContainElement(tssetup.ChoiceProjectMise),
		"an unreadable mise.toml is an unverifiable origin, not an executable one")
	var reason string
	for _, w := range menu.Withheld {
		if w.Kind == tssetup.ChoiceProjectMise {
			reason = w.Reason
		}
	}
	Expect(reason).To(Equal("mise_unverifiable"))
}

func body_projectTsSetupChoiceAcceptanceTest_withholdsProjectPackageForManifestDeclarationThe_273(passingPackageManager projectreadiness.Check, neitherMiseScopeConfigured []projectreadiness.MiseChoice) {
	readiness := projectreadiness.Result{
		Checks: projectreadiness.Checks{
			PackageManager: passingPackageManager,
			Compiler: projectreadiness.Check{
				State: projectreadiness.Fail,
				Code:  projectreadiness.GapTypescriptCompilerMissing,
			},
		},
		MiseChoices: neitherMiseScopeConfigured,
	}

	menu := tssetup.AvailableChoices(readiness)

	Expect(choiceKinds(menu.Choices)).To(Equal([]tssetup.ChoiceKind{tssetup.ChoiceCancel}))
	var projectPackageReason string
	for _, w := range menu.Withheld {
		if w.Kind == tssetup.ChoiceProjectPackage {
			projectPackageReason = w.Reason
		}
	}
	Expect(projectPackageReason).To(Equal("manifest_declaration"))
}
