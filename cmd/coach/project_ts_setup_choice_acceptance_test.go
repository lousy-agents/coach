package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

// AvailableSetupChoices is a pure function over ReadinessResult -- it makes
// no filesystem or network call, so a manifest it is handed can never be
// mutated by it. These specs construct ReadinessResult values directly
// (rather than through the CLI's JSON boundary) because the fields this
// function depends on -- checks.compiler's DeclaredVersion and
// OriginFindings -- are deliberately excluded from that JSON surface
// (json:"-" in project_readiness.go) and reach a customer as rendered
// remediation text only. The exported Go function is therefore the most
// meaningful public boundary available for this behavior today; command
// preview/execution (and any CLI-facing rendering of these choices) is T3/T4.

func choiceKinds(choices []codesignalcli.SetupChoice) []codesignalcli.SetupChoiceKind {
	kinds := make([]codesignalcli.SetupChoiceKind, len(choices))
	for i, c := range choices {
		kinds[i] = c.Kind
	}
	return kinds
}

func withheldKinds(withheld []codesignalcli.WithheldSetupChoice) []codesignalcli.SetupChoiceKind {
	kinds := make([]codesignalcli.SetupChoiceKind, len(withheld))
	for i, w := range withheld {
		kinds[i] = w.Kind
	}
	return kinds
}

var _ = Describe("codesignalcli.AvailableSetupChoices", func() {
	passingPackageManager := codesignalcli.ReadinessCheck{State: codesignalcli.ReadinessPass, Kind: "npm", Version: "11.2.0"}

	allOriginsInstalledElsewhere := []codesignalcli.ReadinessOriginFinding{
		{Origin: "project", Class: "absent"},
		{Origin: "mise_project", Class: "absent"},
		{Origin: "mise_global", Class: "absent"},
	}

	When("the selected manifest declares typescript at a disqualifying (non-exact-in-set) version and nothing is installed", func() {
		It("withholds project-package for that manifest while still offering the mise origins (AC-SET-11)", func() {
			readiness := codesignalcli.ReadinessResult{
				Checks: codesignalcli.ReadinessChecks{
					PackageManager: passingPackageManager,
					Compiler: codesignalcli.ReadinessCheck{
						State:           codesignalcli.ReadinessFail,
						Code:            codesignalcli.GapTypescriptCompilerMissing,
						DeclaredVersion: "^5.0.0",
						OriginFindings:  allOriginsInstalledElsewhere,
					},
				},
			}

			menu := codesignalcli.AvailableSetupChoices(readiness)

			Expect(choiceKinds(menu.Choices)).To(ConsistOf(codesignalcli.SetupChoiceProjectMise, codesignalcli.SetupChoiceGlobalMise, codesignalcli.SetupChoiceCancel),
				"project-package must be absent from the offered choices, mise origins must remain offered")
			Expect(withheldKinds(menu.Withheld)).To(ContainElement(codesignalcli.SetupChoiceProjectPackage))
		})
	})

	When("checks.package_manager is ambiguous because two lockfiles are committed", func() {
		It("requires an explicit selection with no default, and withholds project-package (AC-SET-5)", func() {
			readiness := codesignalcli.ReadinessResult{
				Checks: codesignalcli.ReadinessChecks{
					PackageManager: codesignalcli.ReadinessCheck{State: codesignalcli.ReadinessFail, Code: codesignalcli.GapPackageManagerAmbiguous},
					Compiler: codesignalcli.ReadinessCheck{
						State:          codesignalcli.ReadinessFail,
						Code:           codesignalcli.GapTypescriptCompilerMissing,
						OriginFindings: allOriginsInstalledElsewhere,
					},
				},
			}

			menu := codesignalcli.AvailableSetupChoices(readiness)

			Expect(menu.RequiresExplicitSelection).To(BeTrue(), "an ambiguous package manager must never resolve to a silent default")
			Expect(choiceKinds(menu.Choices)).NotTo(ContainElement(codesignalcli.SetupChoiceProjectPackage))
			Expect(withheldKinds(menu.Withheld)).To(ContainElement(codesignalcli.SetupChoiceProjectPackage))
		})
	})

	When("HEAD carries only Yarn metadata and a verifiable project-mise origin, with no supported compiler installed", func() {
		It("withholds project-package, named, and offers only the mise-family prepare_compiler choices (AC-SET-1 / SA-280-045)", func() {
			readiness := codesignalcli.ReadinessResult{
				Checks: codesignalcli.ReadinessChecks{
					PackageManager: codesignalcli.ReadinessCheck{State: codesignalcli.ReadinessFail, Code: codesignalcli.GapPackageManagerVersionUnsupported, Kind: "yarn"},
					Compiler: codesignalcli.ReadinessCheck{
						State: codesignalcli.ReadinessFail,
						Code:  codesignalcli.GapTypescriptCompilerMissing,
						OriginFindings: []codesignalcli.ReadinessOriginFinding{
							{Origin: "project", Class: "unconfigured"},
							{Origin: "mise_project", Class: "absent"},
							{Origin: "mise_global", Class: "unconfigured"},
						},
					},
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
		})
	})

	When("checks.compiler already passes with an installed supported compiler, alongside Yarn-only metadata", func() {
		It("offers no setup choices at all -- the repository is ready (SA-280-045)", func() {
			readiness := codesignalcli.ReadinessResult{
				Status: codesignalcli.StatusReady,
				Checks: codesignalcli.ReadinessChecks{
					PackageManager: codesignalcli.ReadinessCheck{State: codesignalcli.ReadinessFail, Code: codesignalcli.GapPackageManagerVersionUnsupported, Kind: "yarn"},
					Compiler:       codesignalcli.ReadinessCheck{State: codesignalcli.ReadinessPass, Version: "7.0.2"},
				},
			}

			menu := codesignalcli.AvailableSetupChoices(readiness)

			Expect(menu.Choices).To(BeEmpty())
			Expect(menu.Withheld).To(BeEmpty())
		})
	})

	When("nothing is withheld", func() {
		It("enumerates exactly project-package, project-mise, global-mise, and cancel, in that order", func() {
			readiness := codesignalcli.ReadinessResult{
				Checks: codesignalcli.ReadinessChecks{
					PackageManager: passingPackageManager,
					Compiler: codesignalcli.ReadinessCheck{
						State:          codesignalcli.ReadinessFail,
						Code:           codesignalcli.GapTypescriptCompilerMissing,
						OriginFindings: allOriginsInstalledElsewhere,
					},
				},
			}

			menu := codesignalcli.AvailableSetupChoices(readiness)

			Expect(choiceKinds(menu.Choices)).To(Equal([]codesignalcli.SetupChoiceKind{
				codesignalcli.SetupChoiceProjectPackage,
				codesignalcli.SetupChoiceProjectMise,
				codesignalcli.SetupChoiceGlobalMise,
				codesignalcli.SetupChoiceCancel,
			}))
			Expect(menu.Withheld).To(BeEmpty())
		})
	})

	DescribeTable("never offers project-package when checks.package_manager fails under a rejected code",
		func(rejectedCode string) {
			readiness := codesignalcli.ReadinessResult{
				Checks: codesignalcli.ReadinessChecks{
					PackageManager: codesignalcli.ReadinessCheck{State: codesignalcli.ReadinessFail, Code: rejectedCode, Kind: "npm"},
					Compiler: codesignalcli.ReadinessCheck{
						State:          codesignalcli.ReadinessFail,
						Code:           codesignalcli.GapTypescriptCompilerMissing,
						OriginFindings: allOriginsInstalledElsewhere,
					},
				},
			}

			menu := codesignalcli.AvailableSetupChoices(readiness)

			Expect(choiceKinds(menu.Choices)).NotTo(ContainElement(codesignalcli.SetupChoiceProjectPackage))
			Expect(withheldKinds(menu.Withheld)).To(ContainElement(codesignalcli.SetupChoiceProjectPackage))
		},
		Entry("package_manager_version_unverifiable", codesignalcli.GapPackageManagerVersionUnverifiable),
		Entry("package_manager_version_unsupported", codesignalcli.GapPackageManagerVersionUnsupported),
		Entry("package_manager_config_unverifiable", codesignalcli.GapPackageManagerConfigUnverifiable),
	)
})
