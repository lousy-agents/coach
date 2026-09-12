package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// This suite drives checks.package_manager against the frozen adapter
// support matrix (SA-280-012) purely through --check-project fixtures: a
// recognized manager's metadata (a lockfile or a package.json
// "packageManager" pin) plus, for npm, a committed .npmrc hazard.

var _ = Describe("coach codesignal --baseline --check-project --project-language typescript (package-manager adapter detection)", func() {
	When("a committed .npmrc redirects the npm registry, alongside a recognized npm lockfile and version pin", func() {
		It("reports checks.package_manager fail/package_manager_config_unverifiable and never runs an install", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"npm@11.2.0"}`+"\n")
			commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")
			commitFile(repo, ".npmrc", "registry=https://mirror.example.invalid/npm/\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_config_unverifiable"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("npm"))

			_, statErr := os.Stat(filepath.Join(repo, "node_modules"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "the config hazard must be refused before any install runs, so no node_modules is ever created")
		})
	})

	When("a recognized npm lockfile pins an out-of-range npm major via the packageManager field", func() {
		It("reports checks.package_manager fail/package_manager_version_unsupported with the found version", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"npm@9.5.0"}`+"\n")
			commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_version_unsupported"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("npm"))
			Expect(doc.Checks.PackageManager.FoundVersion).To(Equal("9.5.0"))
			Expect(gapCodes(doc)).To(ContainElement("package_manager_version_unsupported"))
			Expect(doc.Status).To(Equal("needs_prerequisite"))

			var action readinessNextActionDoc
			for _, a := range doc.NextActions {
				if a.Kind == "resolve_package_manager" {
					action = a
				}
			}
			Expect(action.Kind).To(Equal("resolve_package_manager"))
			Expect(action.Executable).To(BeFalse())
			Expect(action.PackageManagerKind).To(Equal("npm"))
			Expect(action.FoundVersion).To(Equal("9.5.0"))

			textStdout, textStderr, textExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript")
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			Expect(string(textStdout)).To(ContainSubstring("resolve_package_manager (executable=false) package_manager_kind=npm found_version=9.5.0"))
		})
	})

	When("a recognized npm lockfile carries no packageManager pin to read a version from", func() {
		It("reports checks.package_manager fail/package_manager_version_unverifiable", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_version_unverifiable"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("npm"))
		})
	})

	When("HEAD carries only Yarn metadata (yarn.lock)", func() {
		It("withholds Yarn, reporting checks.package_manager fail/package_manager_version_unsupported with an informational note, never as an executable choice", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "yarn.lock", "# yarn lockfile v1\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_version_unsupported"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("yarn"))
			Expect(doc.Checks.PackageManager.Detail).NotTo(BeEmpty(), "Yarn's withholding must carry an informational note explaining why")

			Expect(nextActionKinds(doc)).To(ContainElement("resolve_package_manager"), "the executable=false assertion below must run against a next action that actually exists")
			for _, a := range doc.NextActions {
				if a.PackageManagerKind == "yarn" {
					Expect(a.Executable).To(BeFalse(), "Yarn must never be offered as an executable installation choice")
				}
			}
		})
	})

	When("checks.compiler already passes (a supported compiler is installed) alongside Yarn-only metadata", func() {
		It("still reports checks.package_manager fail for information, but contributes no gap entry and no status change (SA-280-045)", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "yarn.lock", "# yarn lockfile v1\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("pass"))
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"), "the manager finding is still reported for information")
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_version_unsupported"))
			Expect(gapCodes(doc)).To(BeEmpty(), "a package-manager finding must never become a gap once a supported compiler already resolves")
			Expect(nextActionKinds(doc)).NotTo(ContainElement("resolve_package_manager"))
			Expect(doc.Status).To(Equal("ready"))
		})
	})

	When("a package.json packageManager field pins npm but no lockfile is committed", func() {
		It("reports checks.package_manager fail/package_manager_config_unverifiable, since npm ci --ignore-scripts requires a committed lockfile", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"npm@11.2.0"}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_config_unverifiable"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("npm"))
		})
	})

	When("committed npm metadata exists but .npmrc is a dangling symlink", func() {
		It("reports checks.package_manager fail/package_manager_config_unverifiable rather than treating an unreadable hazard file as absent", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"npm@11.2.0"}`+"\n")
			commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")
			Expect(os.Symlink(filepath.Join(repo, "does-not-exist-target"), filepath.Join(repo, ".npmrc"))).To(Succeed())

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_config_unverifiable"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("npm"))
		})
	})

	When("a committed .npmrc redirects a scoped registry via the @scope:registry form", func() {
		It("reports checks.package_manager fail/package_manager_config_unverifiable", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"npm@11.2.0"}`+"\n")
			commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")
			commitFile(repo, ".npmrc", "@scope:registry=https://mirror.example.invalid/npm/\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_config_unverifiable"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("npm"))
		})
	})

	When("a package.json packageManager field pins an npm prerelease build inside the numeric major range", func() {
		It("reports checks.package_manager fail/package_manager_version_unsupported, since the frozen range >=11 <12 excludes prereleases", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"npm@11.0.0-rc.1"}`+"\n")
			commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_version_unsupported"))
			Expect(doc.Checks.PackageManager.FoundVersion).To(Equal("11.0.0-rc.1"))
		})
	})

	When("a committed .npmrc re-enables lifecycle scripts using an uppercase boolean value", func() {
		It("reports checks.package_manager pass, since npm's ini parser treats TRUE/True/true identically", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"npm@11.2.0"}`+"\n")
			commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")
			commitFile(repo, ".npmrc", "ignore-scripts=TRUE\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.PackageManager.State).To(Equal("pass"), "an uppercase TRUE must not be misread as a hazardous override")
			Expect(doc.Checks.PackageManager.Kind).To(Equal("npm"))
		})
	})

	When("committed npm metadata exists but package-lock.json is a dangling symlink", func() {
		It("reports checks.package_manager fail/package_manager_config_unverifiable, distinct from a Stat-only presence check", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"npm@11.2.0"}`+"\n")
			Expect(os.Symlink(filepath.Join(repo, "missing-target"), filepath.Join(repo, "package-lock.json"))).To(Succeed())

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_config_unverifiable"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("npm"))
		})
	})

	When("a package.json packageManager field pins pnpm but no pnpm-lock.yaml is committed", func() {
		It("reports checks.package_manager fail/package_manager_config_unverifiable, since installability cannot be verified without a readable lockfile", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"pnpm@10.4.0"}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_config_unverifiable"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("pnpm"))
		})
	})

	When("a recognized npm lockfile and version pin exist with no .npmrc at all", func() {
		It("reports checks.package_manager pass with no package_manager gap", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"npm@11.2.0"}`+"\n")
			commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.PackageManager.State).To(Equal("pass"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("npm"))
			Expect(doc.Checks.PackageManager.Version).To(Equal("11.2.0"))
			for _, code := range gapCodes(doc) {
				Expect(code).NotTo(HavePrefix("package_manager_"))
			}
		})
	})

	When("a recognized pnpm lockfile and supported version pin are committed", func() {
		It("reports checks.package_manager pass", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"pnpm@10.4.0"}`+"\n")
			commitFile(repo, "pnpm-lock.yaml", "lockfileVersion: '9.0'\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.PackageManager.State).To(Equal("pass"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("pnpm"))
			Expect(doc.Checks.PackageManager.Version).To(Equal("10.4.0"))
		})
	})

	When("a recognized pnpm lockfile pins an out-of-range pnpm major", func() {
		It("reports checks.package_manager fail/package_manager_version_unsupported", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"pnpm@9.0.0"}`+"\n")
			commitFile(repo, "pnpm-lock.yaml", "lockfileVersion: '6.0'\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_version_unsupported"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("pnpm"))
			Expect(doc.Checks.PackageManager.FoundVersion).To(Equal("9.0.0"))
		})
	})
})
