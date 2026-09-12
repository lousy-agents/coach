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
})
