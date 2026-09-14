package main

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

// AvailableSetupChoices is a pure function over ReadinessResult -- it makes
// no filesystem or network call, so a manifest it is handed can never be
// mutated by it. These specs construct ReadinessResult values directly
// (rather than through the CLI's JSON boundary) because the fields this
// function depends on -- checks.compiler's DeclaredVersion and the verified
// MiseChoices -- are deliberately excluded from that JSON surface (json:"-"
// in project_readiness.go) and reach a customer as rendered remediation text
// only. The exported Go function is therefore the most meaningful public
// boundary available for this behavior today; BuildSetupPreview and
// ExecuteSetup (project_ts_setup_preview.go, project_ts_setup_execute.go)
// cover command preview/execution, but no CLI-facing rendering of these
// choices exists yet.

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

	// Both mise scopes verified: each already pins an exact supported-set
	// TypeScript version the frozen `mise install` row could realize.
	bothMiseScopesVerified := []codesignalcli.ReadinessMiseChoice{
		{Kind: "mise_project", Verified: true},
		{Kind: "mise_global", Verified: true},
	}

	// Neither scope pins anything installable -- the readiness pipeline
	// reached both and found nothing to install.
	neitherMiseScopeConfigured := []codesignalcli.ReadinessMiseChoice{
		{Kind: "mise_project", Reason: "mise_unconfigured"},
		{Kind: "mise_global", Reason: "mise_unconfigured"},
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
					},
				},
				MiseChoices: bothMiseScopesVerified,
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
						State: codesignalcli.ReadinessFail,
						Code:  codesignalcli.GapTypescriptCompilerMissing,
					},
				},
				MiseChoices: bothMiseScopesVerified,
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
						State:           codesignalcli.ReadinessFail,
						Code:            codesignalcli.GapTypescriptCompilerMissing,
						DeclaredVersion: codesignalcli.SupportedTypescriptVersions[0],
					},
				},
				MiseChoices: bothMiseScopesVerified,
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

	When("an unsupported compiler is installed and the manifest declares a non-exact version", func() {
		It("withholds project-package for the disqualifying declaration (AC-SET-11), even though the failure code is typescript_version_mismatch rather than typescript_compiler_missing", func() {
			readiness := codesignalcli.ReadinessResult{
				Checks: codesignalcli.ReadinessChecks{
					PackageManager: passingPackageManager,
					Compiler: codesignalcli.ReadinessCheck{
						State:           codesignalcli.ReadinessFail,
						Code:            codesignalcli.GapTypescriptVersionMismatch,
						DeclaredVersion: "^5.0.0",
						FoundVersion:    "5.4.0",
					},
				},
			}

			menu := codesignalcli.AvailableSetupChoices(readiness)

			Expect(choiceKinds(menu.Choices)).NotTo(ContainElement(codesignalcli.SetupChoiceProjectPackage),
				"a non-exact manifest declaration disqualifies project-package regardless of which failing compiler code produced it")
			Expect(withheldKinds(menu.Withheld)).To(ContainElement(codesignalcli.SetupChoiceProjectPackage))
		})
	})

	When("checks.compiler fails with typescript_version_conflict and the pipeline reported no mise choices at all", func() {
		It("withholds both mise choices as unverifiable rather than offering them by default (fail-closed)", func() {
			readiness := codesignalcli.ReadinessResult{
				Checks: codesignalcli.ReadinessChecks{
					PackageManager: passingPackageManager,
					Compiler: codesignalcli.ReadinessCheck{
						State: codesignalcli.ReadinessFail,
						Code:  codesignalcli.GapTypescriptVersionConflict,
					},
				},
			}

			menu := codesignalcli.AvailableSetupChoices(readiness)

			Expect(choiceKinds(menu.Choices)).NotTo(ContainElement(codesignalcli.SetupChoiceProjectMise),
				"an ambiguous mise configuration must never be offered as an executable origin merely because evidence is absent")
			Expect(choiceKinds(menu.Choices)).NotTo(ContainElement(codesignalcli.SetupChoiceGlobalMise))
			Expect(withheldKinds(menu.Withheld)).To(ContainElement(codesignalcli.SetupChoiceProjectMise))
			Expect(withheldKinds(menu.Withheld)).To(ContainElement(codesignalcli.SetupChoiceGlobalMise))
		})
	})

	When("checks.compiler fails with typescript_version_mismatch and the pipeline reported no mise choices at all", func() {
		It("withholds both mise choices as unverifiable rather than offering them by default (fail-closed)", func() {
			readiness := codesignalcli.ReadinessResult{
				Checks: codesignalcli.ReadinessChecks{
					PackageManager: passingPackageManager,
					Compiler: codesignalcli.ReadinessCheck{
						State:           codesignalcli.ReadinessFail,
						Code:            codesignalcli.GapTypescriptVersionMismatch,
						DeclaredVersion: "7.0.2",
						FoundVersion:    "5.4.0",
					},
				},
			}

			menu := codesignalcli.AvailableSetupChoices(readiness)

			Expect(choiceKinds(menu.Choices)).NotTo(ContainElement(codesignalcli.SetupChoiceProjectMise))
			Expect(choiceKinds(menu.Choices)).NotTo(ContainElement(codesignalcli.SetupChoiceGlobalMise))
			Expect(withheldKinds(menu.Withheld)).To(ContainElement(codesignalcli.SetupChoiceProjectMise))
			Expect(withheldKinds(menu.Withheld)).To(ContainElement(codesignalcli.SetupChoiceGlobalMise))
		})
	})

	When("the project mise origin's mise.toml exists but could not be read", func() {
		It("withholds project-mise as unverifiable rather than offering an origin Coach could not verify (AC-15)", func() {
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
		})
	})

	When("no mise.toml exists, no global mise typescript pin exists, and the manifest declares nothing", func() {
		It("withholds project-package for manifest_declaration: the frozen rows install what the manifest already declares, and a manifest declaring no typescript cannot install one", func() {
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
		})
	})

	When("the selected manifest declares exactly one exact supported-set typescript version and the lockfile is present", func() {
		It("offers project-package -- this is the one shape where the frozen install command actually lands a supported compiler (AC-1)", func() {
			readiness := codesignalcli.ReadinessResult{
				Checks: codesignalcli.ReadinessChecks{
					PackageManager: passingPackageManager,
					Compiler: codesignalcli.ReadinessCheck{
						State:           codesignalcli.ReadinessFail,
						Code:            codesignalcli.GapTypescriptCompilerMissing,
						DeclaredVersion: codesignalcli.SupportedTypescriptVersions[0],
					},
				},
				MiseChoices: neitherMiseScopeConfigured,
			}

			menu := codesignalcli.AvailableSetupChoices(readiness)

			Expect(choiceKinds(menu.Choices)).To(Equal([]codesignalcli.SetupChoiceKind{
				codesignalcli.SetupChoiceProjectPackage,
				codesignalcli.SetupChoiceCancel,
			}))
			Expect(withheldKinds(menu.Withheld)).To(ConsistOf(codesignalcli.SetupChoiceProjectMise, codesignalcli.SetupChoiceGlobalMise))
		})
	})

	When("no mise.toml exists, no global mise typescript pin exists, and the manifest declares a disqualifying version", func() {
		It("reduces to cancel alone -- there is no origin left that leaves the manifest unmodified (AC-SET-11), and this is intentional, not a bug", func() {
			readiness := codesignalcli.ReadinessResult{
				Checks: codesignalcli.ReadinessChecks{
					PackageManager: passingPackageManager,
					Compiler: codesignalcli.ReadinessCheck{
						State:           codesignalcli.ReadinessFail,
						Code:            codesignalcli.GapTypescriptCompilerMissing,
						DeclaredVersion: "^5.0.0",
					},
				},
				MiseChoices: neitherMiseScopeConfigured,
			}

			menu := codesignalcli.AvailableSetupChoices(readiness)

			Expect(choiceKinds(menu.Choices)).To(Equal([]codesignalcli.SetupChoiceKind{codesignalcli.SetupChoiceCancel}))
			Expect(withheldKinds(menu.Withheld)).To(ConsistOf(
				codesignalcli.SetupChoiceProjectPackage,
				codesignalcli.SetupChoiceProjectMise,
				codesignalcli.SetupChoiceGlobalMise,
			))
		})
	})

	When("checks.compiler already passes and checks.package_manager is ambiguous", func() {
		It("returns the zero menu -- no setup is needed, so no explicit selection can be required either", func() {
			readiness := codesignalcli.ReadinessResult{
				Status: codesignalcli.StatusReady,
				Checks: codesignalcli.ReadinessChecks{
					PackageManager: codesignalcli.ReadinessCheck{State: codesignalcli.ReadinessFail, Code: codesignalcli.GapPackageManagerAmbiguous},
					Compiler:       codesignalcli.ReadinessCheck{State: codesignalcli.ReadinessPass, Version: "7.0.2"},
				},
			}

			menu := codesignalcli.AvailableSetupChoices(readiness)

			Expect(menu.Choices).To(BeEmpty())
			Expect(menu.Withheld).To(BeEmpty())
			Expect(menu.RequiresExplicitSelection).To(BeFalse(), "a ready repository never requires an explicit selection, even if package_manager happens to be ambiguous")
		})
	})

	When("checks.package_manager was never checked because no recognized metadata exists at all", func() {
		It("withholds project-package as unexecutable rather than offering a choice with no package-manager kind to resolve it (AC-1, fail-closed)", func() {
			readiness := codesignalcli.ReadinessResult{
				Checks: codesignalcli.ReadinessChecks{
					PackageManager: codesignalcli.ReadinessCheck{State: codesignalcli.ReadinessNotChecked},
					Compiler: codesignalcli.ReadinessCheck{
						State: codesignalcli.ReadinessFail,
						Code:  codesignalcli.GapTypescriptCompilerMissing,
					},
				},
				MiseChoices: bothMiseScopesVerified,
			}

			menu := codesignalcli.AvailableSetupChoices(readiness)

			Expect(choiceKinds(menu.Choices)).NotTo(ContainElement(codesignalcli.SetupChoiceProjectPackage),
				"an unchecked package manager has no resolved kind for BuildSetupPreview to build a command against, so offering it would be a choice with nothing to execute")
			Expect(withheldKinds(menu.Withheld)).To(ContainElement(codesignalcli.SetupChoiceProjectPackage))
		})
	})

	DescribeTable("never offers project-package when checks.package_manager fails under a rejected code",
		func(rejectedCode string) {
			readiness := codesignalcli.ReadinessResult{
				Checks: codesignalcli.ReadinessChecks{
					PackageManager: codesignalcli.ReadinessCheck{State: codesignalcli.ReadinessFail, Code: rejectedCode, Kind: "npm"},
					Compiler: codesignalcli.ReadinessCheck{
						State: codesignalcli.ReadinessFail,
						Code:  codesignalcli.GapTypescriptCompilerMissing,
					},
				},
				MiseChoices: bothMiseScopesVerified,
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

// The specs above hand AvailableSetupChoices a constructed ReadinessResult.
// These drive the whole pipeline instead -- CheckProjectReadiness, then
// AvailableSetupChoices over its result -- because the question they ask is
// whether the menu's mise entries agree with the mise choices the readiness
// pipeline already verified (evaluateMiseSetupChoices, owner decision D5).
// A constructed result cannot answer that: it is the disagreement between
// the two computations that would be the defect.
var _ = Describe("codesignalcli.AvailableSetupChoices over a real CheckProjectReadiness result", func() {
	When("the project mise.toml pins a typescript version outside the supported set, and nothing is installed", func() {
		It("never offers project_mise, because `mise install` of that pin cannot make checks.compiler pass", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"5.4.0\"\n")
			path, _ := pathWithStatefulStubNodeAndMise("v24.9.9", "5.4.0")
			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))

			revision, err := codesignalcli.ResolveBaselineRevision(repo)
			Expect(err).NotTo(HaveOccurred())
			readiness, err := codesignalcli.CheckProjectReadiness(repo, revision, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(readiness.Checks.Compiler.State).To(Equal(codesignalcli.ReadinessFail), "the fixture must genuinely need setup, or the menu assertion below proves nothing")

			menu := codesignalcli.AvailableSetupChoices(*readiness)

			Expect(choiceKinds(menu.Choices)).NotTo(ContainElement(codesignalcli.SetupChoiceProjectMise),
				"an out-of-set pin is not an installable origin -- consenting to it would leave checks.compiler failing exactly as it was")
			Expect(withheldKinds(menu.Withheld)).To(ContainElement(codesignalcli.SetupChoiceProjectMise))
		})
	})

	When("the project mise.toml pins an exact supported typescript version that is not yet installed", func() {
		It("offers project_mise, so the one shape the frozen `mise install` row can actually resolve stays reachable", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", fmt.Sprintf("[tools]\n\"npm:typescript\" = %q\n", codesignalcli.SupportedTypescriptVersions[0]))
			path, _ := pathWithStatefulStubNodeAndMise("v24.9.9", codesignalcli.SupportedTypescriptVersions[0])
			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))

			revision, err := codesignalcli.ResolveBaselineRevision(repo)
			Expect(err).NotTo(HaveOccurred())
			readiness, err := codesignalcli.CheckProjectReadiness(repo, revision, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(readiness.Checks.Compiler.State).To(Equal(codesignalcli.ReadinessFail))

			menu := codesignalcli.AvailableSetupChoices(*readiness)

			Expect(choiceKinds(menu.Choices)).To(ContainElement(codesignalcli.SetupChoiceProjectMise))
		})
	})
})
