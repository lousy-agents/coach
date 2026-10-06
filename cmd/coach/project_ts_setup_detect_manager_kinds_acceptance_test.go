package main

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --baseline --check-project --project-language typescript (package-manager adapter detection)", func() {
	When("no package.json sits at the repository root and the only selected root is a nested package", func() {
		It("classifies the manager at that root's nearest package.json, the same context resolveCompiler resolves a compiler against", func() {
			repo := newTempGitRepo()
			commitFile(repo, "app/package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "app/package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["app"]}`+"\n")

			path := pathWithStubNodeAndPackageManager("v24.9.9", "npm", "11.4.1")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.PackageManager.State).To(Equal("pass"), "a worktree-root-only scan would find no metadata here and report not_checked")
			Expect(doc.Checks.PackageManager.Kind).To(Equal("npm"))
			Expect(doc.Checks.PackageManager.Version).To(Equal("11.4.1"))
		})
	})

	When("two selected roots resolve to package contexts naming different managers", func() {
		It("reports checks.package_manager fail/package_manager_ambiguous rather than picking one root's manager for both", func() {
			repo := newTempGitRepo()
			commitFile(repo, "app/package.json", `{"name":"app","version":"1.0.0"}`+"\n")
			commitFile(repo, "app/package-lock.json", `{"name":"app","lockfileVersion":3}`+"\n")
			commitFile(repo, "web/package.json", `{"name":"web","version":"1.0.0"}`+"\n")
			commitFile(repo, "web/pnpm-lock.yaml", "lockfileVersion: '9.0'\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["app","web"]}`+"\n")

			path := pathWithStubNodeAndPackageManager("v24.9.9", "npm", "11.4.1")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_ambiguous"))
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

	When("a recognized pnpm lockfile is committed with a supported pnpm on PATH", func() {
		It("reports checks.package_manager pass", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "pnpm-lock.yaml", "lockfileVersion: '9.0'\n")

			path := pathWithStubNodeAndPackageManager("v24.9.9", "pnpm", "10.4.0")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.PackageManager.State).To(Equal("pass"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("pnpm"))
			Expect(doc.Checks.PackageManager.Version).To(Equal("10.4.0"))
		})
	})

	When("a recognized pnpm lockfile is committed and the pnpm on PATH is an out-of-range major", func() {
		It("reports checks.package_manager fail/package_manager_version_unsupported", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "pnpm-lock.yaml", "lockfileVersion: '6.0'\n")

			path := pathWithStubNodeAndPackageManager("v24.9.9", "pnpm", "9.0.0")

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
