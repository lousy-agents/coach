package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

// This suite exercises checks.package_manager's frozen adapter matrix
// (SA-280-012) exhaustively: every manager's version boundary, all three
// failure codes' remediation text, Yarn's withholding, and pnpm/Bun's real
// configuration hazards. Package-manager classification alone is never
// enough to prove a setup choice is withheld -- checks.compiler must also be
// failing, since AvailableSetupChoices returns an empty menu whenever
// checks.compiler already passes (SA-280-045) -- so every fixture below
// omits a typescript devDependency, and each row confirms both the raw
// check and the menu AvailableSetupChoices derives from it.

// resultGapCodes mirrors gapCodes (project_readiness_acceptance_test.go),
// which only reads the JSON-boundary readinessResultDoc; this suite calls
// codesignalcli.CheckProjectReadiness directly so it can also feed the
// result into codesignalcli.AvailableSetupChoices without a JSON round trip.
func resultGapCodes(result codesignalcli.ReadinessResult) []string {
	codes := make([]string, len(result.Gaps))
	for i, gap := range result.Gaps {
		codes[i] = gap.Code
	}
	return codes
}

// packageManagerMatrixFixture commits a minimal repository whose
// package.json declares a supported typescript version without installing it
// -- so checks.compiler always fails here with typescript_compiler_missing
// while the declaration still satisfies appendProjectPackageChoice's offer
// gate, isolating checks.package_manager's own classification as the only
// thing that varies across the matrix below -- plus a single recognized
// lockfile for kind. No packageManager pin is written: the pin no longer
// classifies anything (checkPackageManager probes the binary instead), so a
// fixture that carried one would only obscure which version the row is really
// about.
func packageManagerMatrixFixture(kind string) (repo, head string) {
	repo = newTempGitRepo()
	commitFile(repo, "package.json", fmt.Sprintf(`{"name":"example","version":"1.0.0","devDependencies":{"typescript":"%s"}}`+"\n", codesignalcli.SupportedTypescriptVersions[0]))
	switch kind {
	case "npm":
		head = commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")
	case "pnpm":
		head = commitFile(repo, "pnpm-lock.yaml", "lockfileVersion: '9.0'\n")
	case "bun":
		head = commitFile(repo, "bun.lock", "{\n  \"lockfileVersion\": 0,\n}\n")
	case "yarn":
		head = commitFile(repo, "yarn.lock", "# yarn lockfile v1\n")
	}
	return repo, head
}

// packageManagerMatrixPath puts a stub `kind` reporting version on PATH, or
// -- when version is empty -- a PATH on which no package manager resolves at
// all, which is the matrix's "version undetectable" row rather than a
// separate concept.
func packageManagerMatrixPath(kind, version string) string {
	if version == "" {
		return pathWithStubNode("v24.9.9")
	}
	return pathWithStubNodeAndPackageManager("v24.9.9", kind, version)
}

// expectPackageManagerMatrixRow drives one DescribeTable entry: it asserts
// the fixture genuinely starts with a failing compiler check (or the
// AvailableSetupChoices assertion below would prove nothing), then asserts
// checks.package_manager's classification of the version the manager binary
// on PATH reports and, via AvailableSetupChoices, that an in-matrix version
// is offered as project_package and an out-of-matrix version never is.
func expectPackageManagerMatrixRow(kind, version, wantCode string) {
	GinkgoT().Setenv("PATH", packageManagerMatrixPath(kind, version))
	repo, head := packageManagerMatrixFixture(kind)

	readiness, err := codesignalcli.CheckProjectReadiness(repo, head, "")
	Expect(err).NotTo(HaveOccurred())
	Expect(readiness.Checks.Compiler.State).To(Equal(codesignalcli.ReadinessFail), "the fixture must genuinely fail the compiler check, or the AvailableSetupChoices assertion below proves nothing")
	Expect(readiness.Checks.Compiler.Code).To(Equal(codesignalcli.GapTypescriptCompilerMissing))

	menu := codesignalcli.AvailableSetupChoices(*readiness)

	if wantCode == "" {
		Expect(readiness.Checks.PackageManager.State).To(Equal(codesignalcli.ReadinessPass), "detail=%s", readiness.Checks.PackageManager.Detail)
		Expect(readiness.Checks.PackageManager.Kind).To(Equal(kind))
		Expect(choiceKinds(menu.Choices)).To(ContainElement(codesignalcli.SetupChoiceProjectPackage), "an in-matrix package-manager version must remain offered as a setup choice")
		return
	}

	Expect(readiness.Checks.PackageManager.State).To(Equal(codesignalcli.ReadinessFail))
	Expect(readiness.Checks.PackageManager.Code).To(Equal(wantCode))
	Expect(resultGapCodes(*readiness)).To(ContainElement(wantCode))
	Expect(choiceKinds(menu.Choices)).NotTo(ContainElement(codesignalcli.SetupChoiceProjectPackage), "an out-of-matrix package manager must never be offered as a setup choice")
	Expect(withheldKinds(menu.Withheld)).To(ContainElement(codesignalcli.SetupChoiceProjectPackage))
}

var _ = Describe("checks.package_manager's frozen version-boundary matrix (SA-280-012)", func() {
	DescribeTable("classifies each manager's version against its own frozen supported range, offering an in-matrix version as a project-package setup choice and withholding every out-of-matrix version (AvailableSetupChoices)",
		expectPackageManagerMatrixRow,

		Entry("npm 10.x is below the supported >=11 <12 range", "npm", "10.99.99", codesignalcli.GapPackageManagerVersionUnsupported),
		Entry("npm 11.0.0 is the supported range's low boundary", "npm", "11.0.0", ""),
		Entry("npm 11.99.99 is the supported range's high edge", "npm", "11.99.99", ""),
		Entry("npm 12.0.0 is above the supported range", "npm", "12.0.0", codesignalcli.GapPackageManagerVersionUnsupported),
		Entry("npm absent from PATH leaves its version undetectable", "npm", "", codesignalcli.GapPackageManagerVersionUnverifiable),

		Entry("pnpm 9.x is below the supported >=10 <11 range", "pnpm", "9.99.99", codesignalcli.GapPackageManagerVersionUnsupported),
		Entry("pnpm 10.0.0 is the supported range's low boundary", "pnpm", "10.0.0", ""),
		Entry("pnpm 10.99.99 is the supported range's high edge", "pnpm", "10.99.99", ""),
		Entry("pnpm 11.0.0 is above the supported range", "pnpm", "11.0.0", codesignalcli.GapPackageManagerVersionUnsupported),
		Entry("pnpm absent from PATH leaves its version undetectable", "pnpm", "", codesignalcli.GapPackageManagerVersionUnverifiable),

		Entry("bun below 1.0.0 is outside the supported range", "bun", "0.9.9", codesignalcli.GapPackageManagerVersionUnsupported),
		Entry("bun 1.0.0 is the supported range's low boundary", "bun", "1.0.0", ""),
		Entry("bun's current stable release is supported", "bun", "1.3.11", ""),
		Entry("bun 2.0.0 is above the supported major", "bun", "2.0.0", codesignalcli.GapPackageManagerVersionUnsupported),
		Entry("a prerelease/canary build inside the numeric 1.x range is still excluded", "bun", "1.2.0-canary.20240101", codesignalcli.GapPackageManagerVersionUnsupported),
		Entry("an exact in-range version with no prerelease suffix is accepted as-is", "bun", "1.2.0", ""),
		Entry("bun absent from PATH leaves its version undetectable", "bun", "", codesignalcli.GapPackageManagerVersionUnverifiable),

		Entry("npm build metadata (+build) is excluded, not treated as a plain stable release", "npm", "11.2.0+build.5", codesignalcli.GapPackageManagerVersionUnsupported),
		Entry("pnpm build metadata (+build) is excluded, not treated as a plain stable release", "pnpm", "10.4.0+build.5", codesignalcli.GapPackageManagerVersionUnsupported),
		Entry("bun build/commit metadata (+build) is excluded, not treated as a plain stable release", "bun", "1.2.3+a1b2c3d4", codesignalcli.GapPackageManagerVersionUnsupported),

		Entry("npm reporting a non-semver version is unverifiable, not accepted", "npm", "latest", codesignalcli.GapPackageManagerVersionUnverifiable),
		Entry("pnpm reporting a non-semver version is unverifiable, not accepted", "pnpm", "latest", codesignalcli.GapPackageManagerVersionUnverifiable),
		Entry("bun reporting a non-semver version is unverifiable, not accepted", "bun", "latest", codesignalcli.GapPackageManagerVersionUnverifiable),
	)

	// Corepack writes a sha512 integrity suffix into package.json's
	// packageManager pin (reproduced with real Corepack 0.36.0: `corepack use
	// npm@11.19.0`). That pin attests a release Coach never installs, so it
	// classifies nothing -- the npm on PATH does. This row proves the
	// classification follows the binary even when the pin is the more
	// verifiable-looking of the two.
	When("package.json carries a Corepack integrity pin for a supported npm while an out-of-range npm is on PATH", func() {
		It("classifies by the npm on PATH and records the pin, rather than trusting the attested pin", func() {
			GinkgoT().Setenv("PATH", pathWithStubNodeAndPackageManager("v24.9.9", "npm", "10.9.7"))
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"npm@11.19.0+sha512.48377f8478372aa1c4e47b763475b135836da82436a5700f2e5e8eb5084fc840f93c7b117eb3ad3b5f7d3194c81b6710a10d59448f6ddbcb21ac3fb672bdc003"}`+"\n")
			head := commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")

			readiness, err := codesignalcli.CheckProjectReadiness(repo, head, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(readiness.Checks.PackageManager.State).To(Equal(codesignalcli.ReadinessFail))
			Expect(readiness.Checks.PackageManager.Code).To(Equal(codesignalcli.GapPackageManagerVersionUnsupported))
			Expect(readiness.Checks.PackageManager.FoundVersion).To(Equal("10.9.7"))
			Expect(readiness.Checks.PackageManager.PinnedVersion).To(HavePrefix("11.19.0+sha512."))
		})
	})

	When("HEAD carries only Yarn metadata, and checks.compiler is also failing", func() {
		It("withholds Yarn's package-manager finding as a project-package setup choice, with no default selection (AvailableSetupChoices integration)", func() {
			GinkgoT().Setenv("PATH", pathWithStubNode("v24.9.9"))
			repo, head := packageManagerMatrixFixture("yarn")

			readiness, err := codesignalcli.CheckProjectReadiness(repo, head, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(readiness.Checks.Compiler.State).To(Equal(codesignalcli.ReadinessFail))
			Expect(readiness.Checks.PackageManager.State).To(Equal(codesignalcli.ReadinessFail))
			Expect(readiness.Checks.PackageManager.Code).To(Equal(codesignalcli.GapPackageManagerVersionUnsupported))
			Expect(readiness.Checks.PackageManager.Kind).To(Equal("yarn"))

			menu := codesignalcli.AvailableSetupChoices(*readiness)
			Expect(choiceKinds(menu.Choices)).NotTo(ContainElement(codesignalcli.SetupChoiceProjectPackage), "Yarn must never resolve to an executable project-package choice")
			Expect(withheldKinds(menu.Withheld)).To(ContainElement(codesignalcli.SetupChoiceProjectPackage))
			for _, w := range menu.Withheld {
				if w.Kind == codesignalcli.SetupChoiceProjectPackage {
					Expect(w.Reason).To(Equal(codesignalcli.GapPackageManagerVersionUnsupported))
				}
			}
		})
	})
})

// packageManagerCheckLine returns the single "  package_manager: ..." line
// from RenderReadinessText's output (project_readiness_render_text.go), so a
// spec can compare the actual rendered remediation content rather than only
// the JSON code.
func packageManagerCheckLine(output []byte) string {
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, "package_manager:") {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

var _ = Describe("checks.package_manager's three failure codes render genuinely different remediation text in --check-project's text output", func() {
	It("never collapses package_manager_version_unverifiable, package_manager_version_unsupported, and package_manager_config_unverifiable into one generic line", func() {
		path := pathWithStubNode("v24.9.9")
		unsupportedPath := pathWithStubNodeAndPackageManager("v24.9.9", "npm", "9.5.0")

		unverifiableRepo := newTempGitRepo()
		commitFile(unverifiableRepo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
		commitFile(unverifiableRepo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")
		unverifiableOut, unverifiableErr, unverifiableExit := runCoachCheckProjectEnv(unverifiableRepo, path, "--baseline", "--check-project", "--project-language", "typescript")
		Expect(unverifiableExit).To(Equal(0), "stderr: %s", unverifiableErr)
		unverifiableLine := packageManagerCheckLine(unverifiableOut)

		unsupportedRepo := newTempGitRepo()
		commitFile(unsupportedRepo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
		commitFile(unsupportedRepo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")
		unsupportedOut, unsupportedErr, unsupportedExit := runCoachCheckProjectEnv(unsupportedRepo, unsupportedPath, "--baseline", "--check-project", "--project-language", "typescript")
		Expect(unsupportedExit).To(Equal(0), "stderr: %s", unsupportedErr)
		unsupportedLine := packageManagerCheckLine(unsupportedOut)

		configUnverifiableRepo := newTempGitRepo()
		commitFile(configUnverifiableRepo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
		commitFile(configUnverifiableRepo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")
		commitFile(configUnverifiableRepo, ".npmrc", "registry=https://mirror.example.invalid/npm/\n")
		configUnverifiableOut, configUnverifiableErr, configUnverifiableExit := runCoachCheckProjectEnv(configUnverifiableRepo, path, "--baseline", "--check-project", "--project-language", "typescript")
		Expect(configUnverifiableExit).To(Equal(0), "stderr: %s", configUnverifiableErr)
		configUnverifiableLine := packageManagerCheckLine(configUnverifiableOut)

		Expect(unverifiableLine).To(ContainSubstring("package_manager_version_unverifiable"))
		Expect(unsupportedLine).To(ContainSubstring("package_manager_version_unsupported"))
		Expect(configUnverifiableLine).To(ContainSubstring("package_manager_config_unverifiable"))

		Expect(unverifiableLine).NotTo(Equal(unsupportedLine))
		Expect(unsupportedLine).NotTo(Equal(configUnverifiableLine))
		Expect(unverifiableLine).NotTo(Equal(configUnverifiableLine))

		Expect(unsupportedLine).To(ContainSubstring("found_version=9.5.0"), "the unsupported line must carry the offending version, not a generic phrase")
		Expect(unverifiableLine).NotTo(ContainSubstring("found_version="), "no version was ever found here -- carrying one would misreport the unverifiable case as unsupported")
		Expect(configUnverifiableLine).To(ContainSubstring("detail=committed .npmrc redirects the package registry"), "the config-unverifiable line must name the actual hazard, not a generic phrase")
	})
})

var _ = Describe("checks.package_manager's real pnpm configuration hazards (SA-280-012)", func() {
	When("a committed .npmrc redirects pnpm's package registry", func() {
		It("reports fail/package_manager_config_unverifiable, verified empirically that pnpm honors a committed .npmrc's registry key", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"pnpm@10.4.0"}`+"\n")
			commitFile(repo, "pnpm-lock.yaml", "lockfileVersion: '9.0'\n")
			commitFile(repo, ".npmrc", "registry=https://mirror.example.invalid/npm/\n")

			path := pathWithStubNode("v24.9.9")
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed())
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_config_unverifiable"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("pnpm"))
		})
	})

	When("a committed .npmrc redirects a scoped registry for pnpm via the @scope:registry form", func() {
		It("reports fail/package_manager_config_unverifiable", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"pnpm@10.4.0"}`+"\n")
			commitFile(repo, "pnpm-lock.yaml", "lockfileVersion: '9.0'\n")
			commitFile(repo, ".npmrc", "@scope:registry=https://mirror.example.invalid/npm/\n")

			path := pathWithStubNode("v24.9.9")
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed())
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_config_unverifiable"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("pnpm"))
		})
	})

	// Real pnpm 10.33.0 was tested directly: a postinstall script on a
	// file:-dependency stayed suppressed under this package's frozen `pnpm
	// install --frozen-lockfile --ignore-scripts --ignore-pnpmfile` argv
	// both with a committed .npmrc's enable-pre-post-scripts=true and with
	// that dependency listed in package.json's pnpm.onlyBuiltDependencies
	// allowlist -- neither setting re-enabled it. Refusing on either would
	// be inventing a hazard the frozen argv does not actually have -- the same
	// mistake a Bun trustedDependencies check previously made and had reverted.
	When("a committed .npmrc sets enable-pre-post-scripts=true, verified not to bypass --ignore-scripts", func() {
		It("reports pass, since this setting does not re-enable a suppressed lifecycle script", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"pnpm@10.4.0"}`+"\n")
			commitFile(repo, "pnpm-lock.yaml", "lockfileVersion: '9.0'\n")
			commitFile(repo, ".npmrc", "enable-pre-post-scripts=true\n")

			path := pathWithStubNodeAndPackageManager("v24.9.9", "pnpm", "10.4.0")
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed())
			Expect(doc.Checks.PackageManager.State).To(Equal("pass"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("pnpm"))
		})
	})

	When("package.json's pnpm.onlyBuiltDependencies allowlists a dependency's build script, verified not to bypass --ignore-scripts", func() {
		It("reports pass, since pnpm's own build-script allowlist never overrides --ignore-scripts", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","pnpm":{"onlyBuiltDependencies":["some-dependency"]}}`+"\n")
			commitFile(repo, "pnpm-lock.yaml", "lockfileVersion: '9.0'\n")

			path := pathWithStubNodeAndPackageManager("v24.9.9", "pnpm", "10.4.0")
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed())
			Expect(doc.Checks.PackageManager.State).To(Equal("pass"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("pnpm"))
		})
	})
})

var _ = Describe("checks.package_manager's real Bun configuration hazards (SA-280-012)", func() {
	When("a committed bunfig.toml redirects the install registry", func() {
		It("reports fail/package_manager_config_unverifiable, verified empirically that `bun install --frozen-lockfile --ignore-scripts` still requests packages from the redirected registry", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"bun@1.3.11"}`+"\n")
			commitFile(repo, "bun.lock", "{\n  \"lockfileVersion\": 0,\n}\n")
			commitFile(repo, "bunfig.toml", "[install]\nregistry = \"https://mirror.example.invalid/npm/\"\n")

			path := pathWithStubNode("v24.9.9")
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed())
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_config_unverifiable"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("bun"))
		})
	})

	When("a committed bunfig.toml redirects a scoped install registry under [install.scopes]", func() {
		It("reports fail/package_manager_config_unverifiable", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"bun@1.3.11"}`+"\n")
			commitFile(repo, "bun.lock", "{\n  \"lockfileVersion\": 0,\n}\n")
			commitFile(repo, "bunfig.toml", "[install.scopes]\n\"@example\" = \"https://mirror.example.invalid/npm/\"\n")

			path := pathWithStubNode("v24.9.9")
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed())
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_config_unverifiable"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("bun"))
		})
	})

	// Verified by real subprocess execution that bunfig.toml's "preload"
	// entry never fires on `bun install` (only on `bun run`/the bun
	// runtime). Refusing on it here would reintroduce a hazard already
	// proven not to exist.
	When("a committed bunfig.toml sets a preload hook but no registry redirection", func() {
		It("reports pass, since preload does not fire on `bun install`", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"bun@1.3.11"}`+"\n")
			commitFile(repo, "bun.lock", "{\n  \"lockfileVersion\": 0,\n}\n")
			commitFile(repo, "bunfig.toml", "preload = [\"./setup.ts\"]\n")

			path := pathWithStubNodeAndPackageManager("v24.9.9", "bun", "1.3.11")
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed())
			Expect(doc.Checks.PackageManager.State).To(Equal("pass"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("bun"))
		})
	})

	// Verified empirically against real Bun 1.3.11: `bun install
	// --ignore-scripts` still refuses to reach left-pad's real registry
	// (ConnectionRefused against a redirected host) from a committed .npmrc
	// alone, with NO bunfig.toml present at all -- Bun's registry resolution
	// honors .npmrc the same way npm/pnpm do, on top of bunfig.toml, not
	// instead of it.
	When("a committed .npmrc redirects the registry, with no bunfig.toml present", func() {
		It("reports fail/package_manager_config_unverifiable, since Bun honors .npmrc's registry key even without a bunfig.toml", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"bun@1.3.11"}`+"\n")
			commitFile(repo, "bun.lock", "{\n  \"lockfileVersion\": 0,\n}\n")
			commitFile(repo, ".npmrc", "registry=https://mirror.example.invalid/npm/\n")

			path := pathWithStubNode("v24.9.9")
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed())
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_config_unverifiable"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("bun"))
		})
	})

	When("a committed .npmrc redirects a scoped registry for Bun via the @scope:registry form, with no bunfig.toml present", func() {
		It("reports fail/package_manager_config_unverifiable, verified empirically against real Bun 1.3.11", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"bun@1.3.11"}`+"\n")
			commitFile(repo, "bun.lock", "{\n  \"lockfileVersion\": 0,\n}\n")
			commitFile(repo, ".npmrc", "@fortawesome:registry=https://mirror.example.invalid/npm/\n")

			path := pathWithStubNode("v24.9.9")
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed())
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_config_unverifiable"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("bun"))
		})
	})

	When("committed bun metadata exists but bunfig.toml is a dangling symlink", func() {
		It("reports checks.package_manager fail/package_manager_config_unverifiable rather than treating an unreadable hazard file as absent", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"bun@1.3.11"}`+"\n")
			commitFile(repo, "bun.lock", "{\n  \"lockfileVersion\": 0,\n}\n")
			Expect(os.Symlink(filepath.Join(repo, "does-not-exist-target"), filepath.Join(repo, "bunfig.toml"))).To(Succeed())

			path := pathWithStubNode("v24.9.9")
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed())
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_config_unverifiable"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("bun"))
		})
	})

	// Real Bun 1.3.11 was reproduced against ALL FOUR of these header
	// spellings via ConnectionRefused on a redirected registry: bunfig.toml
	// is TOML, and TOML legally permits whitespace inside a table header's
	// brackets and a quoted (basic/literal string) table name -- a
	// byte-equality check against the bare lowercase name evades detection
	// for the spaced and quoted spellings while Bun itself still honors them.
	DescribeTable("honors bunfig.toml's [install] and [install.scopes] headers regardless of legal TOML whitespace/quoting spelling",
		func(bunfigContent string) {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"bun@1.3.11"}`+"\n")
			commitFile(repo, "bun.lock", "{\n  \"lockfileVersion\": 0,\n}\n")
			commitFile(repo, "bunfig.toml", bunfigContent)

			path := pathWithStubNode("v24.9.9")
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed())
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_config_unverifiable"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("bun"))
		},
		Entry("a spaced [ install ] header", "[ install ]\nregistry = \"https://mirror.example.invalid/npm/\"\n"),
		Entry("a quoted [\"install\"] header", "[\"install\"]\nregistry = \"https://mirror.example.invalid/npm/\"\n"),
		Entry("a spaced [ install.scopes ] header", "[ install.scopes ]\n\"@example\" = \"https://mirror.example.invalid/npm/\"\n"),
		Entry("a quoted [\"install.scopes\"] header", "[\"install.scopes\"]\n\"@example\" = \"https://mirror.example.invalid/npm/\"\n"),
	)

	// The first three entries below were reproduced against real Bun 1.3.11
	// with a redirected registry: each still yields ConnectionRefused (the
	// redirect is honored), which the header-whitespace/quoting fix above
	// does not cover. A trailing comment after a table header, a UTF-8 BOM
	// at the file's start, and TOML's inline-table syntax are all ordinary,
	// legal TOML -- not adversarial edge cases. The last two entries
	// reproduce a third, more severe gap: a committed [install.cache] dir
	// redirect was verified empirically to make `bun install
	// --frozen-lockfile --ignore-scripts` silently install tampered file
	// content from the redirected cache dir -- no network fetch, no
	// integrity failure, and no lifecycle script
	// needed -- rather than fail to connect.
	DescribeTable("honors bunfig.toml spellings a line-oriented header scan alone would miss",
		func(bunfigContent string) {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"bun@1.3.11"}`+"\n")
			commitFile(repo, "bun.lock", "{\n  \"lockfileVersion\": 0,\n}\n")
			commitFile(repo, "bunfig.toml", bunfigContent)

			path := pathWithStubNode("v24.9.9")
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed())
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_config_unverifiable"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("bun"))
		},
		Entry("an [install] header with a trailing comment", "[install] # hi\nregistry = \"https://mirror.example.invalid/npm/\"\n"),
		Entry("a UTF-8 BOM before the first header", "\xEF\xBB\xBF[install]\nregistry = \"https://mirror.example.invalid/npm/\"\n"),
		Entry("an inline table spelling install.registry", "install = { registry = \"https://mirror.example.invalid/npm/\" }\n"),
		Entry("a scopes key under an [install] header", "[install]\nscopes = { \"@types\" = \"http://127.0.0.1:1/\" }\n"),
		Entry("an inline table spelling install.scopes with no [install] header at all", "install = { scopes = { \"@types\" = \"http://127.0.0.1:1/\" } }\n"),
		Entry("an [install.cache] header redirecting the cache dir", "[install.cache]\ndir = \"./.bun-cache\"\n"),
		Entry("an inline table spelling install.cache.dir with no [install] header at all", "install = { cache = { dir = \"./.bun-cache\" } }\n"),
		Entry("a \\u-escaped [install] table name redirecting the registry", "[\"insta\\u006Cl\"]\nregistry = \"http://127.0.0.1:1/\"\n"),
		Entry("a \\u-escaped [install].cache table name redirecting the cache dir", "[\"insta\\u006Cl\".cache]\ndir = \"./.bun-cache\"\n"),
		Entry("a \\x-escaped [install] table name and escaped registry key", "[\"insta\\x6Cl\"]\n\"regist\\x72y\" = \"http://127.0.0.1:1/\"\n"),
		Entry("an octal-escaped [install] table name and escaped registry key", "[\"insta\\154l\"]\n\"regist\\162y\" = \"http://127.0.0.1:1/\"\n"),
		// dir is deliberately "./.evil-dir", not "./.bun-cache" like the sibling
		// entries above -- "bun-cache" itself contains the literal substring
		// "cache", which the keyword backstop would catch by accident,
		// masking the actual escape-evasion gap this entry exists to prove.
		Entry("a \\x-escaped [install.cache] table name redirecting the cache dir", "[\"insta\\x6Cl\".\"cach\\x65\"]\ndir = \"./.evil-dir\"\n"),
	)

	When("a committed bunfig.toml sets a preload hook but the scanner's structured pass finds no registry key", func() {
		It("still reports pass, so the fail-closed install/registry/scopes/cache-keyword and \\u-escape backstop does not false-positive on an ordinary preload-only file", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"bun@1.3.11"}`+"\n")
			commitFile(repo, "bun.lock", "{\n  \"lockfileVersion\": 0,\n}\n")
			commitFile(repo, "bunfig.toml", "preload = [\"./setup.ts\"]\n")

			path := pathWithStubNodeAndPackageManager("v24.9.9", "bun", "1.3.11")
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed())
			Expect(doc.Checks.PackageManager.State).To(Equal("pass"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("bun"))
		})
	})
})

var _ = Describe("checks.package_manager's ambiguous-lockfile and lifecycle-script-hazard fixtures", func() {
	When("two lockfiles of different managers are committed with no packageManager field to disambiguate", func() {
		It("reports fail/package_manager_ambiguous and requires an explicit selection with no default project-package choice (AC-5)", func() {
			GinkgoT().Setenv("PATH", pathWithStubNode("v24.9.9"))
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")
			head := commitFile(repo, "pnpm-lock.yaml", "lockfileVersion: '9.0'\n")

			readiness, err := codesignalcli.CheckProjectReadiness(repo, head, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(readiness.Checks.Compiler.State).To(Equal(codesignalcli.ReadinessFail))
			Expect(readiness.Checks.PackageManager.State).To(Equal(codesignalcli.ReadinessFail))
			Expect(readiness.Checks.PackageManager.Code).To(Equal(codesignalcli.GapPackageManagerAmbiguous))
			Expect(resultGapCodes(*readiness)).To(ContainElement(codesignalcli.GapPackageManagerAmbiguous))

			menu := codesignalcli.AvailableSetupChoices(*readiness)
			Expect(menu.RequiresExplicitSelection).To(BeTrue(), "an ambiguous package manager must never resolve to a silent default")
			Expect(choiceKinds(menu.Choices)).NotTo(ContainElement(codesignalcli.SetupChoiceProjectPackage))
			Expect(withheldKinds(menu.Withheld)).To(ContainElement(codesignalcli.SetupChoiceProjectPackage))
		})
	})

	When("a committed .npmrc sets ignore-scripts=false", func() {
		It("reports fail/package_manager_config_unverifiable", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"npm@11.2.0"}`+"\n")
			commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")
			commitFile(repo, ".npmrc", "ignore-scripts=false\n")

			path := pathWithStubNode("v24.9.9")
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed())
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_config_unverifiable"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("npm"))
			Expect(doc.Checks.PackageManager.Detail).To(ContainSubstring("re-enables lifecycle scripts"), "a mis-wired shared helper must not be able to report this hazard's code with the wrong detail text")
		})
	})

	When("a committed .npmrc overrides the lifecycle script shell", func() {
		It("reports fail/package_manager_config_unverifiable", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"npm@11.2.0"}`+"\n")
			commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")
			commitFile(repo, ".npmrc", "script-shell=/bin/sh\n")

			path := pathWithStubNode("v24.9.9")
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed())
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_config_unverifiable"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("npm"))
			Expect(doc.Checks.PackageManager.Detail).To(ContainSubstring("overrides the lifecycle script shell"), "a mis-wired shared helper must not be able to report this hazard's code with the wrong detail text")
		})
	})
})
