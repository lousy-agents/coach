package main

import (
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

func body_projectTsSetupChoiceAcceptanceTest_withholdsProjectPackageNamedAndOffersOnlyTheMise_103() {
	readiness := codesignalcli.ReadinessResult{
		Checks: codesignalcli.ReadinessChecks{
			PackageManager: codesignalcli.ReadinessCheck{State: codesignalcli.ReadinessFail, Code: codesignalcli.GapPackageManagerVersionUnsupported, Kind: "yarn"},
			Compiler: codesignalcli.ReadinessCheck{
				State: codesignalcli.ReadinessFail,
				Code:  codesignalcli.GapTypescriptCompilerMissing,
			},
		},
		MiseChoices: []codesignalcli.ReadinessMiseChoice{
			{Kind: "mise_project", Verified: true},
			{Kind: "mise_global", Reason: "mise_unconfigured"},
		},
	}

	menu := codesignalcli.AvailableSetupChoices(readiness)

	Expect(choiceKinds(menu.Choices)).To(Equal([]codesignalcli.SetupChoiceKind{codesignalcli.SetupChoiceProjectMise, codesignalcli.SetupChoiceCancel}))

	var projectPackageReason string
	for _, w := range menu.Withheld {
		if w.Kind == codesignalcli.SetupChoiceProjectPackage {
			projectPackageReason = w.Reason
		}
	}
	Expect(projectPackageReason).To(Equal(codesignalcli.GapPackageManagerVersionUnsupported), "the project adapter must be named, not merely omitted")
}

func body_projectTsSetupChoiceAcceptanceTest_withholdsProjectMiseAsUnverifiableRatherThanOffe_243(passingPackageManager codesignalcli.ReadinessCheck) {
	readiness := codesignalcli.ReadinessResult{
		Checks: codesignalcli.ReadinessChecks{
			PackageManager: passingPackageManager,
			Compiler: codesignalcli.ReadinessCheck{
				State: codesignalcli.ReadinessFail,
				Code:  codesignalcli.GapTypescriptCompilerMissing,
			},
		},
		MiseChoices: []codesignalcli.ReadinessMiseChoice{
			{Kind: "mise_project", Reason: "mise_unverifiable"},
			{Kind: "mise_global", Verified: true},
		},
	}

	menu := codesignalcli.AvailableSetupChoices(readiness)

	Expect(choiceKinds(menu.Choices)).NotTo(ContainElement(codesignalcli.SetupChoiceProjectMise),
		"an unreadable mise.toml is an unverifiable origin, not an executable one")
	var reason string
	for _, w := range menu.Withheld {
		if w.Kind == codesignalcli.SetupChoiceProjectMise {
			reason = w.Reason
		}
	}
	Expect(reason).To(Equal("mise_unverifiable"))
}

func body_projectTsSetupChoiceAcceptanceTest_withholdsProjectPackageForManifestDeclarationThe_273(passingPackageManager codesignalcli.ReadinessCheck, neitherMiseScopeConfigured []codesignalcli.ReadinessMiseChoice) {
	readiness := codesignalcli.ReadinessResult{
		Checks: codesignalcli.ReadinessChecks{
			PackageManager: passingPackageManager,
			Compiler: codesignalcli.ReadinessCheck{
				State: codesignalcli.ReadinessFail,
				Code:  codesignalcli.GapTypescriptCompilerMissing,
			},
		},
		MiseChoices: neitherMiseScopeConfigured,
	}

	menu := codesignalcli.AvailableSetupChoices(readiness)

	Expect(choiceKinds(menu.Choices)).To(Equal([]codesignalcli.SetupChoiceKind{codesignalcli.SetupChoiceCancel}))
	var projectPackageReason string
	for _, w := range menu.Withheld {
		if w.Kind == codesignalcli.SetupChoiceProjectPackage {
			projectPackageReason = w.Reason
		}
	}
	Expect(projectPackageReason).To(Equal("manifest_declaration"))
}
